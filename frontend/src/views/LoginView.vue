<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { errorMessage } from '../api/client'
import { useAuth } from '../composables/useAuth'

const props = defineProps<{ setup: boolean }>()
const auth = useAuth()
const submitting = ref(false)
const form = reactive({ username: '', displayName: '', password: '', confirmPassword: '' })

const title = computed(() => (props.setup ? '创建初始管理员' : '用户登录'))

async function submit(): Promise<void> {
  const username = form.username.trim()
  if (!username || !form.password) {
    ElMessage.error('请输入用户名和密码')
    return
  }
  if (props.setup && form.password.length < 8) {
    ElMessage.error('密码至少需要 8 个字符')
    return
  }
  if (props.setup && form.password !== form.confirmPassword) {
    ElMessage.error('两次输入的密码不一致')
    return
  }
  submitting.value = true
  try {
    if (props.setup) {
      await auth.createAdmin(username, form.displayName.trim(), form.password)
      ElMessage.success('管理员已创建')
    } else {
      await auth.signIn(username, form.password)
    }
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <el-card class="login-card" shadow="always">
      <div class="brand-mark">CNC</div>
      <h1>CNC 加工程序管理系统</h1>
      <p class="subtitle">{{ title }}</p>
      <el-form label-position="top" @submit.prevent="submit">
        <el-form-item label="用户名">
          <el-input v-model="form.username" maxlength="64" autocomplete="username" autofocus />
        </el-form-item>
        <el-form-item v-if="setup" label="显示名称">
          <el-input v-model="form.displayName" maxlength="64" placeholder="留空则使用用户名" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            :autocomplete="setup ? 'new-password' : 'current-password'"
            @keyup.enter="submit"
          />
        </el-form-item>
        <el-form-item v-if="setup" label="确认密码">
          <el-input
            v-model="form.confirmPassword"
            type="password"
            show-password
            autocomplete="new-password"
            @keyup.enter="submit"
          />
        </el-form-item>
        <el-button class="submit-button" type="primary" :loading="submitting" @click="submit">
          {{ setup ? '创建并进入系统' : '登录' }}
        </el-button>
      </el-form>
      <p v-if="setup" class="hint">这是首次启动。请创建管理员账号，密码至少 8 个字符。</p>
    </el-card>
  </div>
</template>

<style scoped>
.login-page {
  display: grid;
  min-height: 100%;
  place-items: center;
  padding: 24px;
  background: linear-gradient(145deg, #e8f1fb 0%, #f5f7fa 48%, #e8edf4 100%);
}

.login-card {
  width: min(420px, 100%);
  border: 0;
  border-radius: 12px;
}

.brand-mark {
  width: 58px;
  margin: 4px auto 14px;
  border-radius: 8px;
  background: var(--el-color-primary);
  color: white;
  font-size: 18px;
  font-weight: 700;
  line-height: 46px;
  text-align: center;
}

h1 {
  margin: 0;
  font-size: 22px;
  text-align: center;
}

.subtitle {
  margin: 8px 0 24px;
  color: var(--el-text-color-secondary);
  text-align: center;
}

.submit-button {
  width: 100%;
  margin-top: 6px;
}

.hint {
  margin: 18px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
  text-align: center;
}
</style>
