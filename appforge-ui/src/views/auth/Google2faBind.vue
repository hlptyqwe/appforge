<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { apiGoogle2faBind, apiGoogle2faInit } from '@/api/system/users'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()

const loading = ref(false)
const initLoading = ref(false)
const form = reactive({
  code: '',
})
const initData = reactive({
  secret: '',
  otpauthUrl: '',
  qrCode: '',
})

async function loadGoogle2fa() {
  const userId = Number(auth.user?.id || 0)
  if (!userId) return

  initLoading.value = true
  try {
    const res = await apiGoogle2faInit({ userId })
    if (res.code !== 200) throw new Error(res.msg || 'init failed')
    initData.secret = res.data?.secret || ''
    initData.otpauthUrl = res.data?.otpauthUrl || ''
    initData.qrCode = res.data?.qrCode || ''
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
  } finally {
    initLoading.value = false
  }
}

async function copy(value: string) {
  if (!value) return
  await navigator.clipboard.writeText(value)
  ElMessage.success(t('common.copySuccess') || t('common.success'))
}

async function submit() {
  if (!form.code) {
    ElMessage.warning(t('common.pleaseInputCode'))
    return
  }
  if (!initData.secret) {
    await loadGoogle2fa()
  }
  if (!initData.secret) return

  const userId = Number(auth.user?.id || 0)
  if (!userId) return

  loading.value = true
  try {
    const res = await apiGoogle2faBind({
      userId,
      secret: initData.secret,
      code: form.code,
    })
    if (res.code !== 200) throw new Error(res.msg || 'bind failed')

    ElMessage.success(t('common.success'))
    await auth.fetchProfile()
    if (auth.user) {
      auth.user.google2FaEnabled = 1
    }
    await router.replace('/home')
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
  } finally {
    loading.value = false
  }
}

onMounted(loadGoogle2fa)
</script>

<template>
  <div class="google2fa-bind-page">
    <div class="bind-panel">
      <div class="bind-main">
        <h1>{{ t('auth.google2faRequiredTitle') }}</h1>
        <p>{{ t('auth.google2faRequiredDesc') }}</p>

        <el-form label-position="top" class="bind-form" @submit.prevent>
          <el-form-item :label="t('common.code')">
            <el-input
              v-model="form.code"
              :placeholder="t('common.enterGoogleCode')"
              maxlength="6"
              @keyup.enter="submit"
            />
          </el-form-item>

          <el-form-item :label="t('common.secret')">
            <div class="field-row">
              <el-input :model-value="initData.secret" readonly />
              <el-button :disabled="!initData.secret" @click="copy(initData.secret)">
                {{ t('common.copy') }}
              </el-button>
            </div>
          </el-form-item>

          <el-form-item :label="t('common.otpauthUrl')">
            <div class="field-row">
              <el-input :model-value="initData.otpauthUrl" readonly />
              <el-button :disabled="!initData.otpauthUrl" @click="copy(initData.otpauthUrl)">
                {{ t('common.copy') }}
              </el-button>
            </div>
          </el-form-item>

          <el-button
            type="primary"
            :loading="loading"
            class="submit-button"
            @click="submit"
          >
            {{ t('auth.bindGoogle2fa') }}
          </el-button>
        </el-form>
      </div>

      <div class="qr-panel">
        <div class="qr-title">
          {{ t('common.qrCode') }}
        </div>
        <div v-loading="initLoading" class="qr-box">
          <img v-if="initData.qrCode" :src="initData.qrCode" alt="Google 2FA QR code">
          <span v-else>{{ t('common.click2faBindGenerateQrCode') }}</span>
        </div>
        <div class="qr-tip">
          {{ t('common.scanQrCodeWithGoogleAuthenticator') }}
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.google2fa-bind-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fb;
  padding: 32px;
}

.bind-panel {
  width: min(920px, 100%);
  display: grid;
  grid-template-columns: minmax(0, 1fr) 300px;
  gap: 32px;
  background: #fff;
  border: 1px solid #e5e7ef;
  border-radius: 8px;
  box-shadow: 0 12px 32px rgba(32, 45, 64, 0.08);
  padding: 32px;
}

.bind-main h1 {
  font-size: 28px;
  line-height: 1.2;
  margin: 0 0 12px;
  color: #242934;
}

.bind-main p {
  margin: 0 0 28px;
  color: #6b7280;
}

.bind-form {
  max-width: 520px;
}

.field-row {
  width: 100%;
  display: flex;
  gap: 8px;
}

.field-row .el-input {
  flex: 1;
}

.submit-button {
  width: 100%;
  margin-top: 8px;
}

.qr-panel {
  border-left: 1px solid #edf0f5;
  padding-left: 32px;
}

.qr-title {
  font-weight: 600;
  margin-bottom: 12px;
  color: #242934;
}

.qr-box {
  min-height: 236px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f7f8fa;
  border: 1px solid #e5e7ef;
  border-radius: 8px;
  padding: 12px;
  color: #909399;
}

.qr-box img {
  width: 100%;
  height: auto;
  display: block;
}

.qr-tip {
  margin-top: 10px;
  font-size: 13px;
  color: #6b7280;
}

@media (max-width: 760px) {
  .google2fa-bind-page {
    padding: 16px;
  }

  .bind-panel {
    grid-template-columns: 1fr;
    padding: 24px;
  }

  .qr-panel {
    border-left: 0;
    border-top: 1px solid #edf0f5;
    padding-left: 0;
    padding-top: 24px;
  }
}
</style>
