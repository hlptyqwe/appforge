<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import CursorPagination from '@/components/common/CursorPagination.vue'
import { usePagination } from '@/composables/usePagination'
import {
  platformService,
  type PlatformWhiteLabelTemplate,
  type PlatformWhiteLabelTemplateRevision,
} from '@/services'

const rows = ref<PlatformWhiteLabelTemplate[]>([])
const revisions = ref<PlatformWhiteLabelTemplateRevision[]>([])
const loading = ref(false)
const submitting = ref(false)
const createVisible = ref(false)
const revisionsVisible = ref(false)
const revisionVisible = ref(false)
const copyVisible = ref(false)
const activeTemplate = ref<PlatformWhiteLabelTemplate>()
const editingTemplateId = ref(0)
const editingRevision = ref(0)
const schemaMode = ref<'designer' | 'json'>('designer')
const schemaFields = ref<
  Array<{ name: string; type: string; required: boolean; sensitive: boolean }>
>([])
const selectedTemplateFile = ref<File>()
const templateFileProgress = ref(0)
const templateFileTarget = ref('assets/appforge/customer.json')
const templateFileOperation = ref<'resource.replaceFile' | 'extension.writeValidatedFile'>(
  'extension.writeValidatedFile',
)
const formRef = ref<FormInstance>()
const query = reactive({ appId: 0, keyword: '', status: 0 })
const form = reactive({
  appId: 0,
  templateCode: '',
  templateName: '',
  sourceVersionId: 0,
  parameterSchemaJson:
    '{\n  "type": "object",\n  "properties": {},\n  "additionalProperties": false\n}',
  compatibilityRulesJson: '{}',
})
const revisionForm = reactive({
  packageNameRuleJson: '{"mode":"explicit"}',
  manifestPatchJson: '[{"op":"manifest.setPackage"}]',
  resourcePatchJson: '[]',
  extensionFilesJson: '[]',
  expectedArtifactsJson: '{"verifyPackageName":true,"verifySignature":true}',
})
const copyForm = reactive({ templateCode: '', templateName: '', sourceVersionId: 0 })
const { pagination, updateFromResponse, resetAndLoad, nextAndLoad, prevAndLoad } =
  usePagination<number>(20)

const templateStatus = (value: number) => ({ 1: '草稿', 2: '已发布', 3: '停用' })[value] || '未知'
const revisionStatus = (value: number) => ({ 1: '草稿', 2: '已发布', 3: '已取代' })[value] || '未知'
const templateDialogTitle = computed(() =>
  editingTemplateId.value ? '编辑白标模板' : '新增白标模板',
)

function validateJSONFields(values: Record<string, unknown>, fields: string[]) {
  for (const field of fields) JSON.parse(String(values[field] || ''))
}

async function loadData() {
  loading.value = true
  try {
    const response = await platformService.listWhiteLabelTemplates({
      ...query,
      cursor: pagination.cursor,
      limit: pagination.limit,
    })
    if (response.code !== 200) throw new Error(response.msg)
    rows.value = response.data || []
    updateFromResponse(response)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载白标模板失败')
  } finally {
    loading.value = false
  }
}

function loadSchemaDesigner(raw: string) {
  schemaFields.value = []
  try {
    const schema = JSON.parse(raw) as {
      properties?: Record<string, { type?: string; sensitive?: boolean }>
      required?: string[]
    }
    schemaFields.value = Object.entries(schema.properties || {}).map(([name, definition]) => ({
      name,
      type: definition.type || 'string',
      required: (schema.required || []).includes(name),
      sensitive: Boolean(definition.sensitive),
    }))
  } catch {
    schemaMode.value = 'json'
  }
}

function syncSchemaFromDesigner() {
  const properties: Record<string, { type: string; sensitive?: boolean }> = {}
  const required: string[] = []
  for (const field of schemaFields.value) {
    const name = field.name.trim()
    if (!name) continue
    if (properties[name]) throw new Error(`参数 ${name} 重复`)
    if (field.sensitive && field.type !== 'string')
      throw new Error(`敏感参数 ${name} 只能使用 string 类型`)
    properties[name] = { type: field.type, ...(field.sensitive ? { sensitive: true } : {}) }
    if (field.required) required.push(name)
  }
  form.parameterSchemaJson = JSON.stringify(
    { type: 'object', properties, required, additionalProperties: false },
    null,
    2,
  )
}

function openTemplateForm(row?: PlatformWhiteLabelTemplate) {
  editingTemplateId.value = row?.id || 0
  Object.assign(form, {
    appId: row?.appId || 0,
    templateCode: row?.templateCode || '',
    templateName: row?.templateName || '',
    sourceVersionId: row?.sourceVersionId || 0,
    parameterSchemaJson:
      row?.parameterSchemaJson ||
      '{\n  "type": "object",\n  "properties": {},\n  "additionalProperties": false\n}',
    compatibilityRulesJson: row?.compatibilityRulesJson || '{}',
  })
  schemaMode.value = 'designer'
  loadSchemaDesigner(form.parameterSchemaJson)
  createVisible.value = true
}

async function saveTemplate() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    if (schemaMode.value === 'designer') syncSchemaFromDesigner()
    validateJSONFields(form, ['parameterSchemaJson', 'compatibilityRulesJson'])
    const response = editingTemplateId.value
      ? await platformService.updateWhiteLabelTemplate(editingTemplateId.value, { ...form })
      : await platformService.createWhiteLabelTemplate({ ...form })
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success(editingTemplateId.value ? '模板已更新' : '模板已创建')
    createVisible.value = false
    await resetAndLoad(loadData)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '创建模板失败')
  } finally {
    submitting.value = false
  }
}

function openCopy(row: PlatformWhiteLabelTemplate) {
  activeTemplate.value = row
  Object.assign(copyForm, {
    templateCode: `${row.templateCode}-copy`,
    templateName: `${row.templateName} 副本`,
    sourceVersionId: row.sourceVersionId,
  })
  copyVisible.value = true
}

async function copyTemplate() {
  if (!activeTemplate.value) return
  submitting.value = true
  try {
    const response = await platformService.copyWhiteLabelTemplate(activeTemplate.value.id, {
      ...copyForm,
    })
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success('模板及修订已复制为草稿')
    copyVisible.value = false
    await resetAndLoad(loadData)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '复制模板失败')
  } finally {
    submitting.value = false
  }
}

async function deleteTemplate(row: PlatformWhiteLabelTemplate) {
  try {
    await ElMessageBox.confirm('仅未发布且未被产品引用的草稿模板可以删除，是否继续？', '删除确认')
    const response = await platformService.deleteWhiteLabelTemplate(row.id)
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success('模板已删除')
    await resetAndLoad(loadData)
  } catch (error) {
    if (error !== 'cancel' && error !== 'close')
      ElMessage.error(error instanceof Error ? error.message : '删除模板失败')
  }
}

async function showRevisions(row: PlatformWhiteLabelTemplate) {
  activeTemplate.value = row
  revisionsVisible.value = true
  const response = await platformService.listWhiteLabelTemplateRevisions(row.id, { limit: 100 })
  if (response.code !== 200) {
    ElMessage.error(response.msg || '加载模板修订失败')
    return
  }
  revisions.value = response.data || []
}

function openRevision(row?: PlatformWhiteLabelTemplateRevision) {
  editingRevision.value = row?.revision || 0
  Object.assign(revisionForm, {
    packageNameRuleJson: row?.packageNameRuleJson || '{"mode":"explicit"}',
    manifestPatchJson: row?.manifestPatchJson || '[{"op":"manifest.setPackage"}]',
    resourcePatchJson: row?.resourcePatchJson || '[]',
    extensionFilesJson: row?.extensionFilesJson || '[]',
    expectedArtifactsJson:
      row?.expectedArtifactsJson || '{"verifyPackageName":true,"verifySignature":true}',
  })
  selectedTemplateFile.value = undefined
  templateFileProgress.value = 0
  revisionVisible.value = true
}

async function saveRevision() {
  if (!activeTemplate.value) return
  submitting.value = true
  try {
    validateJSONFields(revisionForm, [
      'packageNameRuleJson',
      'manifestPatchJson',
      'resourcePatchJson',
      'extensionFilesJson',
      'expectedArtifactsJson',
    ])
    const response = editingRevision.value
      ? await platformService.updateWhiteLabelTemplateRevision(
          activeTemplate.value.id,
          editingRevision.value,
          { ...revisionForm },
        )
      : await platformService.createWhiteLabelTemplateRevision(activeTemplate.value.id, {
          ...revisionForm,
        })
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success(
      `修订 ${response.data?.revision} 已${editingRevision.value ? '更新' : '创建'}`,
    )
    revisionVisible.value = false
    await showRevisions(activeTemplate.value)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '创建模板修订失败')
  } finally {
    submitting.value = false
  }
}

async function deleteRevision(row: PlatformWhiteLabelTemplateRevision) {
  if (!activeTemplate.value) return
  try {
    await ElMessageBox.confirm(`确定删除草稿修订 ${row.revision} 吗？`, '删除确认')
    const response = await platformService.deleteWhiteLabelTemplateRevision(
      activeTemplate.value.id,
      row.revision,
    )
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success('草稿修订已删除')
    await showRevisions(activeTemplate.value)
  } catch (error) {
    if (error !== 'cancel' && error !== 'close')
      ElMessage.error(error instanceof Error ? error.message : '删除修订失败')
  }
}

function selectTemplateFile(event: Event) {
  selectedTemplateFile.value = (event.target as HTMLInputElement).files?.[0]
}

async function uploadTemplateFile() {
  if (!activeTemplate.value || !selectedTemplateFile.value) {
    ElMessage.warning('请先选择模板文件')
    return
  }
  const target = templateFileTarget.value.trim()
  if (!target) {
    ElMessage.warning('请输入 APK 内目标路径')
    return
  }
  submitting.value = true
  try {
    const response = await platformService.uploadObject(
      selectedTemplateFile.value,
      7,
      activeTemplate.value.appId,
      (value) => (templateFileProgress.value = value),
    )
    if (response.code !== 200 || !response.data) throw new Error(response.msg)
    const operation = {
      op: templateFileOperation.value,
      path: target,
      objectId: response.data.objectId,
    }
    const field: 'resourcePatchJson' | 'extensionFilesJson' =
      templateFileOperation.value === 'resource.replaceFile'
        ? 'resourcePatchJson'
        : 'extensionFilesJson'
    const operations = JSON.parse(revisionForm[field] || '[]') as Array<Record<string, unknown>>
    operations.push(operation)
    revisionForm[field] = JSON.stringify(operations, null, 2)
    ElMessage.success(`文件已私有上传并绑定对象 ${response.data.objectId}`)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '模板文件上传失败')
  } finally {
    submitting.value = false
  }
}

async function publishRevision(row: PlatformWhiteLabelTemplateRevision) {
  if (!activeTemplate.value) return
  try {
    await ElMessageBox.confirm(`发布修订 ${row.revision} 后内容将不可修改，是否继续？`, '发布确认')
    const response = await platformService.publishWhiteLabelTemplate(
      activeTemplate.value.id,
      row.revision,
    )
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success('模板修订已发布')
    await showRevisions(response.data!)
    await loadData()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close')
      ElMessage.error(error instanceof Error ? error.message : '发布失败')
  }
}

async function changeTemplateStatus(row: PlatformWhiteLabelTemplate) {
  const nextStatus = row.status === 3 ? 2 : 3
  try {
    await ElMessageBox.confirm(
      nextStatus === 3 ? '停用后关联产品将无法新建构建，是否继续？' : '确定重新启用该模板吗？',
      '状态确认',
    )
    const response = await platformService.changeWhiteLabelTemplateStatus(row.id, nextStatus)
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success('模板状态已更新')
    await loadData()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close')
      ElMessage.error(error instanceof Error ? error.message : '状态更新失败')
  }
}

onMounted(loadData)
</script>

<template>
  <div class="module-page">
    <el-card shadow="never" class="query-card">
      <el-form inline>
        <el-form-item label="应用 ID"
          ><el-input-number v-model="query.appId" :min="0"
        /></el-form-item>
        <el-form-item label="关键词"><el-input v-model="query.keyword" clearable /></el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" style="width: 130px">
            <el-option label="全部" :value="0" /><el-option label="草稿" :value="1" />
            <el-option label="已发布" :value="2" /><el-option label="停用" :value="3" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="resetAndLoad(loadData)">查询</el-button>
          <el-button v-perm="'core:white-label-template:add'" @click="openTemplateForm()"
            >新增</el-button
          >
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="rows" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="appId" label="应用 ID" width="100" />
        <el-table-column prop="templateCode" label="模板编码" min-width="150" />
        <el-table-column prop="templateName" label="模板名称" min-width="160" />
        <el-table-column prop="sourceVersionId" label="源版本 ID" width="110" />
        <el-table-column prop="publishedRevision" label="发布修订" width="100" />
        <el-table-column label="状态" width="100"
          ><template #default="scope">{{
            templateStatus(scope.row.status)
          }}</template></el-table-column
        >
        <el-table-column label="操作" width="360" fixed="right">
          <template #default="scope"
            ><el-button link type="primary" @click="showRevisions(scope.row)">修订管理</el-button
            ><el-button
              v-if="scope.row.status === 1 && scope.row.publishedRevision === 0"
              v-perm="'core:white-label-template:update'"
              link
              type="primary"
              @click="openTemplateForm(scope.row)"
              >编辑</el-button
            ><el-button v-perm="'core:white-label-template:copy'" link @click="openCopy(scope.row)"
              >复制</el-button
            ><el-button
              v-if="scope.row.publishedRevision > 0"
              v-perm="'core:white-label-template:status'"
              link
              @click="changeTemplateStatus(scope.row)"
              >{{ scope.row.status === 3 ? '启用' : '停用' }}</el-button
            ><el-button
              v-if="scope.row.status === 1 && scope.row.publishedRevision === 0"
              v-perm="'core:white-label-template:delete'"
              link
              type="danger"
              @click="deleteTemplate(scope.row)"
              >删除</el-button
            ></template
          >
        </el-table-column>
      </el-table>
      <CursorPagination
        v-model:limit="pagination.limit"
        :total="pagination.total"
        :has-prev="pagination.hasPrev"
        :has-next="pagination.hasNext"
        @prev="prevAndLoad(loadData)"
        @next="nextAndLoad(loadData)"
        @limit-change="resetAndLoad(loadData)"
      />
    </el-card>

    <el-dialog v-model="createVisible" :title="templateDialogTitle" width="820px">
      <el-form ref="formRef" :model="form" label-width="150px">
        <el-form-item
          label="应用 ID"
          prop="appId"
          :rules="[{ required: true, message: '请输入应用 ID' }]"
          ><el-input-number
            v-model="form.appId"
            :min="1"
            style="width: 100%"
            :disabled="Boolean(editingTemplateId)"
        /></el-form-item>
        <el-form-item
          label="模板编码"
          prop="templateCode"
          :rules="[{ required: true, message: '请输入模板编码' }]"
          ><el-input
            v-model="form.templateCode"
            placeholder="lowercase-template-code"
            :disabled="Boolean(editingTemplateId)"
        /></el-form-item>
        <el-form-item
          label="模板名称"
          prop="templateName"
          :rules="[{ required: true, message: '请输入模板名称' }]"
          ><el-input v-model="form.templateName"
        /></el-form-item>
        <el-form-item
          label="源版本 ID"
          prop="sourceVersionId"
          :rules="[{ required: true, message: '请输入源版本 ID' }]"
          ><el-input-number v-model="form.sourceVersionId" :min="1" style="width: 100%"
        /></el-form-item>
        <el-form-item label="参数 Schema 设计">
          <el-radio-group v-model="schemaMode">
            <el-radio-button value="designer">可视化</el-radio-button>
            <el-radio-button value="json">JSON</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="schemaMode === 'designer'" label="参数字段">
          <div class="schema-designer">
            <div v-for="(field, index) in schemaFields" :key="index" class="schema-row">
              <el-input v-model="field.name" placeholder="参数名" />
              <el-select v-model="field.type">
                <el-option
                  v-for="type in ['string', 'boolean', 'integer', 'number', 'object', 'array']"
                  :key="type"
                  :label="type"
                  :value="type"
                />
              </el-select>
              <el-checkbox v-model="field.required">必填</el-checkbox>
              <el-checkbox v-model="field.sensitive" :disabled="field.type !== 'string'"
                >敏感</el-checkbox
              >
              <el-button link type="danger" @click="schemaFields.splice(index, 1)">删除</el-button>
            </div>
            <el-button
              link
              type="primary"
              @click="schemaFields.push({ name: '', type: 'string', required: false, sensitive: false })"
              >+ 添加参数</el-button
            >
          </div>
        </el-form-item>
        <el-form-item v-else label="参数 JSON Schema"
          ><el-input v-model="form.parameterSchemaJson" type="textarea" :rows="8"
        /></el-form-item>
        <el-form-item label="兼容规则 JSON"
          ><el-input v-model="form.compatibilityRulesJson" type="textarea" :rows="4"
        /></el-form-item>
      </el-form>
      <template #footer
        ><el-button @click="createVisible = false">取消</el-button
        ><el-button type="primary" :loading="submitting" @click="saveTemplate"
          >保存</el-button
        ></template
      >
    </el-dialog>

    <el-dialog v-model="copyVisible" title="复制白标模板" width="620px">
      <el-form :model="copyForm" label-width="130px">
        <el-form-item label="新模板编码"><el-input v-model="copyForm.templateCode" /></el-form-item>
        <el-form-item label="新模板名称"><el-input v-model="copyForm.templateName" /></el-form-item>
        <el-form-item label="源版本 ID"
          ><el-input-number v-model="copyForm.sourceVersionId" :min="1" style="width: 100%"
        /></el-form-item>
      </el-form>
      <el-alert
        title="复制后的模板和全部修订均为草稿，需要重新发布。"
        type="info"
        :closable="false"
      />
      <template #footer
        ><el-button @click="copyVisible = false">取消</el-button
        ><el-button type="primary" :loading="submitting" @click="copyTemplate"
          >复制</el-button
        ></template
      >
    </el-dialog>

    <el-dialog
      v-model="revisionsVisible"
      :title="`${activeTemplate?.templateName || ''} · 修订管理`"
      width="1100px"
    >
      <div class="dialog-toolbar">
        <el-button
          v-perm="'core:white-label-template:revision'"
          type="primary"
          @click="openRevision()"
          >新增修订</el-button
        >
      </div>
      <el-table :data="revisions" max-height="520">
        <el-table-column prop="revision" label="修订" width="80" />
        <el-table-column prop="checksum" label="Checksum" min-width="290" show-overflow-tooltip />
        <el-table-column
          prop="manifestPatchJson"
          label="Manifest 补丁"
          min-width="260"
          show-overflow-tooltip
        />
        <el-table-column label="状态" width="100"
          ><template #default="scope">{{
            revisionStatus(scope.row.status)
          }}</template></el-table-column
        >
        <el-table-column label="操作" width="190"
          ><template #default="scope"
            ><el-button
              v-if="scope.row.status === 1"
              v-perm="'core:white-label-template:revision'"
              link
              type="primary"
              @click="openRevision(scope.row)"
              >编辑</el-button
            ><el-button
              v-if="scope.row.status === 1"
              v-perm="'core:white-label-template:publish'"
              link
              type="success"
              @click="publishRevision(scope.row)"
              >发布</el-button
            ><el-button
              v-if="scope.row.status === 1"
              v-perm="'core:white-label-template:revision'"
              link
              type="danger"
              @click="deleteRevision(scope.row)"
              >删除</el-button
            ></template
          ></el-table-column
        >
      </el-table>
    </el-dialog>

    <el-dialog
      v-model="revisionVisible"
      :title="editingRevision ? `编辑草稿修订 ${editingRevision}` : '新增不可变修订'"
      width="900px"
    >
      <el-form :model="revisionForm" label-width="170px">
        <el-form-item label="包名规则 JSON"
          ><el-input v-model="revisionForm.packageNameRuleJson" type="textarea" :rows="2"
        /></el-form-item>
        <el-form-item label="Manifest 补丁 JSON"
          ><el-input v-model="revisionForm.manifestPatchJson" type="textarea" :rows="5"
        /></el-form-item>
        <el-form-item label="资源补丁 JSON"
          ><el-input v-model="revisionForm.resourcePatchJson" type="textarea" :rows="4"
        /></el-form-item>
        <el-form-item label="扩展文件 JSON"
          ><el-input v-model="revisionForm.extensionFilesJson" type="textarea" :rows="4"
        /></el-form-item>
        <el-form-item label="产物验收 JSON"
          ><el-input v-model="revisionForm.expectedArtifactsJson" type="textarea" :rows="3"
        /></el-form-item>
        <el-divider content-position="left">受控模板文件</el-divider>
        <el-form-item label="操作类型">
          <el-select v-model="templateFileOperation" style="width: 100%">
            <el-option label="替换现有 res/ 文件" value="resource.replaceFile" />
            <el-option label="写入 assets/ 或 res/raw/" value="extension.writeValidatedFile" />
          </el-select>
        </el-form-item>
        <el-form-item label="APK 内目标路径"
          ><el-input v-model="templateFileTarget" placeholder="assets/appforge/customer.json"
        /></el-form-item>
        <el-form-item label="选择文件">
          <div class="template-file-upload">
            <input type="file" accept=".json,.xml,.txt,.png,.webp" @change="selectTemplateFile" />
            <el-button :loading="submitting" @click="uploadTemplateFile"
              >私有上传并加入补丁</el-button
            >
            <el-progress v-if="templateFileProgress > 0" :percentage="templateFileProgress" />
          </div>
        </el-form-item>
      </el-form>
      <el-alert
        title="仅允许声明式补丁；命令、脚本、外部下载和路径穿越会被服务端拒绝。"
        type="warning"
        :closable="false"
      />
      <template #footer
        ><el-button @click="revisionVisible = false">取消</el-button
        ><el-button type="primary" :loading="submitting" @click="saveRevision"
          >保存修订</el-button
        ></template
      >
    </el-dialog>
  </div>
</template>

<style scoped>
.dialog-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}

.schema-designer,
.template-file-upload {
  display: flex;
  width: 100%;
  flex-direction: column;
  gap: 10px;
}

.schema-row {
  display: grid;
  grid-template-columns: minmax(160px, 1fr) 150px 70px 70px 52px;
  gap: 10px;
  align-items: center;
}
</style>
