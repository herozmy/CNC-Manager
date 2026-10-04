<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import LoginView from './views/LoginView.vue'
import { useAuth } from './composables/useAuth'

const auth = useAuth()
const { user, ready, setupRequired } = auth
const handleUnauthorized = (): void => auth.clearUser()

onMounted(() => {
  window.addEventListener('cnccool:unauthorized', handleUnauthorized)
  void auth.initialize()
})

onBeforeUnmount(() => window.removeEventListener('cnccool:unauthorized', handleUnauthorized))
</script>

<template>
  <div v-if="!ready" class="app-loading" v-loading="true" />
  <LoginView v-else-if="!user" :setup="setupRequired" />
  <router-view v-else />
</template>

<style scoped>
.app-loading {
  height: 100%;
}
</style>
