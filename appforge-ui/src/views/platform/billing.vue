<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { IS_AGENT } from '@/config/environment'
import {
  platformService,
  type PlatformBillingPlan,
  type PlatformInvoice,
  type PlatformTenantBilling,
  type PlatformUsageMetricSummary,
} from '@/services'

const loading = ref(false)
const plans = ref<PlatformBillingPlan[]>([])
const billing = ref<PlatformTenantBilling>()
const usage = ref<PlatformUsageMetricSummary[]>([])
const periodKey = ref('')
const invoices = ref<PlatformInvoice[]>([])
const invoicePage = ref(1)
const invoiceTotal = ref(0)
const tenantId = ref(0)
const planDialog = ref(false)
const contractDialog = ref(false)
const planForm = reactive({
  planCode: '', planName: '', billingCycle: 1, priceAmount: 0, currency: 'CNY', featureJson: '{}',
  buildsPerCycle: 100, maxBuildConcurrency: 1, storageBytes: 1073741824,
  maxUploadBytes: 268435456, teamSeats: 3, apiRateLimit: 60,
  chargeFailedBuild: false, chargeCacheHit: true, chargeRetryBuild: true,
})
const contractForm = reactive({ planId: 0, periodStart: Date.now(), periodEnd: Date.now() + 30 * 86400000, contractReference: '', overrideJson: '' })

const subscriptionStatus = computed(() => ({ 1: '生效', 2: '逾期', 3: '宽限', 4: '暂停', 5: '已取消', 6: '待支付' }[billing.value?.subscription.status || 0] || '未开通'))
const usageNames: Record<number, string> = { 1: '构建次数', 2: '成功构建', 3: '构建计算秒数', 4: '源文件存储', 5: '产物存储', 6: '日志存储', 7: 'Open API 请求', 8: '团队席位' }
const invoiceStatuses: Record<number, string> = { 1: '草稿', 2: '待支付', 3: '已支付', 4: '支付失败', 5: '已作废', 6: '已退款' }

function money(value: number, currency = 'CNY') {
  return new Intl.NumberFormat('zh-CN', { style: 'currency', currency }).format(value / 100)
}
function quota(value: number) { return value < 0 ? '不限量' : value.toLocaleString() }
function date(value?: number) { return value ? new Date(value).toLocaleString() : '-' }

async function loadPlans() {
  const response = await platformService.listBillingPlans({ page: 1, pageSize: 100, status: 1 })
  if (response.code !== 200) throw new Error(response.msg)
  plans.value = response.data || []
}
async function loadTenantData() {
  if (!IS_AGENT && tenantId.value <= 0) return
  const target = IS_AGENT ? 0 : tenantId.value
  const [billingResponse, usageResponse, invoiceResponse] = await Promise.all([
    platformService.getTenantBilling(target),
    platformService.getBillingUsage(target ? { tenantId: target } : {}),
    platformService.listInvoices({ tenantId: target || undefined, page: invoicePage.value, pageSize: 20 }),
  ])
  if (billingResponse.code !== 200) throw new Error(billingResponse.msg)
  billing.value = { subscription: billingResponse.subscription, entitlement: billingResponse.entitlement, plan: billingResponse.plan }
  usage.value = usageResponse.data || []
  periodKey.value = usageResponse.periodKey || ''
  invoices.value = invoiceResponse.data || []
  invoiceTotal.value = invoiceResponse.total || 0
}
async function load() {
  loading.value = true
  try { await loadPlans(); await loadTenantData() }
  catch (error) { ElMessage.error(error instanceof Error ? error.message : '账务数据加载失败') }
  finally { loading.value = false }
}
async function checkout(plan: PlatformBillingPlan) {
  try {
    if (plan.priceAmount === 0) {
      const response = await platformService.changeSubscription(plan.id, 1)
      if (response.code !== 200) throw new Error(response.msg)
      ElMessage.success('套餐已切换'); await loadTenantData(); return
    }
    const response = await platformService.createBillingCheckout(plan.id)
    if (response.code !== 200 || !response.checkoutUrl) throw new Error(response.msg)
    window.location.assign(response.checkoutUrl)
  } catch (error) { ElMessage.error(error instanceof Error ? error.message : '结账失败') }
}
async function cancelSubscription() {
  await ElMessageBox.confirm('订阅将在当前周期结束时取消，是否继续？', '取消订阅')
  const response = await platformService.cancelSubscription(false)
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success('已安排周期末取消'); await loadTenantData()
}
async function savePlan() {
  const response = await platformService.createBillingPlan({ ...planForm })
  if (response.code !== 200) return ElMessage.error(response.msg)
  planDialog.value = false; ElMessage.success('套餐版本已创建'); await loadPlans()
}
async function retirePlan(id: number) {
  await ElMessageBox.confirm('退役后不能用于新订阅，历史订阅不受影响。', '退役套餐')
  const response = await platformService.retireBillingPlan(id)
  if (response.code !== 200) throw new Error(response.msg)
  await loadPlans()
}
async function saveContract() {
  const response = await platformService.upsertManualSubscription({ tenantId: tenantId.value, ...contractForm })
  if (response.code !== 200) return ElMessage.error(response.msg)
  contractDialog.value = false; ElMessage.success('人工合同已生效'); await loadTenantData()
}

onMounted(load)
</script>

<template>
  <div v-loading="loading" class="billing-page">
    <el-card v-if="!IS_AGENT" shadow="never" class="query-card">
      <el-form inline @submit.prevent>
        <el-form-item label="租户 ID"><el-input-number v-model="tenantId" :min="1" /></el-form-item>
        <el-form-item><el-button type="primary" @click="loadTenantData">查询租户</el-button><el-button @click="planDialog = true">新增套餐版本</el-button><el-button :disabled="tenantId <= 0" @click="contractDialog = true">人工合同</el-button></el-form-item>
      </el-form>
    </el-card>

    <el-card v-if="billing" shadow="never">
      <el-descriptions :column="4" border>
        <el-descriptions-item label="当前套餐">{{ billing.plan.planName }} v{{ billing.plan.version }}</el-descriptions-item>
        <el-descriptions-item label="订阅状态">{{ subscriptionStatus }}</el-descriptions-item>
        <el-descriptions-item label="本周期截止">{{ date(billing.subscription.currentPeriodEnd) }}</el-descriptions-item>
        <el-descriptions-item label="构建并发">{{ quota(billing.entitlement.maxBuildConcurrency) }}</el-descriptions-item>
      </el-descriptions>
      <div v-if="IS_AGENT" class="card-actions"><el-button v-if="!billing.subscription.cancelAtPeriodEnd" type="danger" plain @click="cancelSubscription">周期末取消</el-button><el-tag v-else type="warning">已安排周期末取消</el-tag></div>
    </el-card>

    <div class="plan-grid">
      <el-card v-for="plan in plans" :key="plan.id" shadow="never">
        <div class="plan-name">{{ plan.planName }} <el-tag size="small">v{{ plan.version }}</el-tag></div>
        <div class="plan-price">{{ money(plan.priceAmount, plan.currency) }}<small>/{{ plan.billingCycle === 2 ? '年' : '月' }}</small></div>
        <ul><li>构建 {{ quota(plan.buildsPerCycle) }}</li><li>并发 {{ quota(plan.maxBuildConcurrency) }}</li><li>存储 {{ plan.storageBytes < 0 ? '不限量' : Math.round(plan.storageBytes / 1073741824) + ' GiB' }}</li><li>席位 {{ quota(plan.teamSeats) }}</li><li>API {{ quota(plan.apiRateLimit) }} 次/分钟</li></ul>
        <el-button v-if="IS_AGENT" type="primary" :disabled="billing?.subscription.planId === plan.id" @click="checkout(plan)">{{ billing?.subscription.planId === plan.id ? '当前套餐' : '选择套餐' }}</el-button>
        <el-button v-else type="danger" plain @click="retirePlan(plan.id)">退役</el-button>
      </el-card>
    </div>

    <el-card v-if="billing" shadow="never">
      <template #header>用量 {{ periodKey }}</template>
      <el-table :data="usage" stripe><el-table-column label="指标"><template #default="scope">{{ usageNames[scope.row.metric] }}</template></el-table-column><el-table-column prop="usedQuantity" label="已使用" /><el-table-column prop="reservedQuantity" label="预占" /><el-table-column label="限额"><template #default="scope">{{ quota(scope.row.limitQuantity) }}</template></el-table-column><el-table-column label="使用率" min-width="180"><template #default="scope"><el-progress v-if="scope.row.limitQuantity > 0" :percentage="Math.min(scope.row.usagePercent, 100)" /><span v-else>—</span></template></el-table-column></el-table>
    </el-card>

    <el-card v-if="billing" shadow="never" class="invoice-card">
      <template #header>账单</template>
      <el-table :data="invoices" stripe height="100%"><el-table-column prop="invoiceNo" label="账单号" min-width="180" /><el-table-column label="周期" min-width="260"><template #default="scope">{{ date(scope.row.periodStart) }} - {{ date(scope.row.periodEnd) }}</template></el-table-column><el-table-column label="金额"><template #default="scope">{{ money(scope.row.totalAmount, scope.row.currency) }}</template></el-table-column><el-table-column label="状态"><template #default="scope">{{ invoiceStatuses[scope.row.status] }}</template></el-table-column><el-table-column label="支付时间"><template #default="scope">{{ date(scope.row.paidAt) }}</template></el-table-column></el-table>
      <div class="pagination"><el-pagination v-model:current-page="invoicePage" :page-size="20" :total="invoiceTotal" layout="total, prev, pager, next" @current-change="loadTenantData" /></div>
    </el-card>

    <el-dialog v-model="planDialog" title="新增不可变套餐版本" width="620px"><el-form label-width="130px"><el-form-item label="套餐编码"><el-input v-model="planForm.planCode" /></el-form-item><el-form-item label="名称"><el-input v-model="planForm.planName" /></el-form-item><el-form-item label="周期"><el-radio-group v-model="planForm.billingCycle"><el-radio :value="1">月付</el-radio><el-radio :value="2">年付</el-radio></el-radio-group></el-form-item><el-form-item label="价格（分）"><el-input-number v-model="planForm.priceAmount" :min="0" /></el-form-item><el-form-item label="构建次数"><el-input-number v-model="planForm.buildsPerCycle" :min="-1" /></el-form-item><el-form-item label="最大并发"><el-input-number v-model="planForm.maxBuildConcurrency" :min="-1" /></el-form-item><el-form-item label="存储字节"><el-input-number v-model="planForm.storageBytes" :min="-1" /></el-form-item><el-form-item label="单文件字节"><el-input-number v-model="planForm.maxUploadBytes" :min="-1" /></el-form-item><el-form-item label="团队席位"><el-input-number v-model="planForm.teamSeats" :min="-1" /></el-form-item><el-form-item label="API 每分钟"><el-input-number v-model="planForm.apiRateLimit" :min="-1" /></el-form-item></el-form><template #footer><el-button @click="planDialog = false">取消</el-button><el-button type="primary" @click="savePlan">创建版本</el-button></template></el-dialog>
    <el-dialog v-model="contractDialog" title="人工合同订阅" width="560px"><el-form label-width="110px"><el-form-item label="套餐"><el-select v-model="contractForm.planId"><el-option v-for="plan in plans" :key="plan.id" :label="`${plan.planName} v${plan.version}`" :value="plan.id" /></el-select></el-form-item><el-form-item label="合同编号"><el-input v-model="contractForm.contractReference" /></el-form-item><el-form-item label="开始时间"><el-date-picker v-model="contractForm.periodStart" type="datetime" value-format="x" /></el-form-item><el-form-item label="结束时间"><el-date-picker v-model="contractForm.periodEnd" type="datetime" value-format="x" /></el-form-item><el-form-item label="权益覆盖"><el-input v-model="contractForm.overrideJson" type="textarea" placeholder='{"buildsPerCycle":2000}' /></el-form-item></el-form><template #footer><el-button @click="contractDialog = false">取消</el-button><el-button type="primary" @click="saveContract">生效</el-button></template></el-dialog>
  </div>
</template>

<style scoped>
.billing-page { display: flex; min-height: calc(100vh - 96px); flex-direction: column; gap: 16px; }
.plan-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: 16px; }
.plan-name { font-size: 18px; font-weight: 600; }.plan-price { margin: 18px 0; font-size: 28px; color: var(--el-color-primary); }.plan-price small { font-size: 13px; color: var(--el-text-color-secondary); }.plan-grid ul { min-height: 132px; padding-left: 20px; line-height: 2; color: var(--el-text-color-regular); }.card-actions { display: flex; justify-content: flex-end; margin-top: 16px; }.invoice-card { flex: 1; min-height: 360px; }.invoice-card :deep(.el-card__body) { display: flex; height: calc(100% - 58px); flex-direction: column; }.pagination { display: flex; justify-content: flex-end; padding-top: 16px; }
</style>
