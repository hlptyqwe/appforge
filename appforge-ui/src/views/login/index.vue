<script setup lang="ts">
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { resolvePostLoginPath } from '@/router'
import { useI18n } from 'vue-i18n'
import { useForm, useLoading } from '@/composables'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const { loading, withLoading } = useLoading()

const { form } = useForm({
  initialData: {
    username: '',
    password: '',
    googleCode: '',
  },
})

async function submit() {
  await withLoading(async () => {
    try {
      await auth.login({
        username: form.username,
        password: form.password,
        googleCode: form.googleCode || undefined,
      })
      await auth.fetchProfile()
      const requestedRedirect = Array.isArray(route.query.redirect)
        ? route.query.redirect[0] || ''
        : String(route.query.redirect || '')
      await router.replace(resolvePostLoginPath(requestedRedirect, auth.menus))
    } catch (e: unknown) {
      alert(e)
    }
  })
}
</script>

<template>
  <div class="wrap">
    <el-card class="card">
      <template #header>
        {{ t('route.login') }}
      </template>

      <el-form label-position="top">
        <el-form-item :label="t('auth.username')">
          <el-input v-model="form.username" autocomplete="username" />
        </el-form-item>

        <el-form-item :label="t('auth.password')">
          <el-input
            v-model="form.password"
            type="password"
            autocomplete="current-password"
            show-password
          />
        </el-form-item>

        <el-form-item :label="t('auth.googleCode')">
          <el-input v-model="form.googleCode" />
        </el-form-item>

        <el-button
          type="primary"
          :loading="loading"
          style="width: 100%"
          @click="submit"
        >
          {{ t('auth.submit') }}
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.wrap {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f7f8fa;
}
.card {
  width: 380px;
}
</style>
