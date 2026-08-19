<template>
  <div class="login-layout login-layout--gateway">
    <header class="auth-topbar">
      <div class="header-logo" title="FTMind">
        <img :src="brandLogo" class="logo-image" alt="FTMind" />
        <span class="logo-wordmark">FTMind</span>
      </div>

      <div class="auth-system">
        <span class="auth-system__gateway">FTMIND AUTH GATEWAY · 01</span>
        <span class="auth-system__status"><i></i>SYSTEM READY</span>
        <div class="language-switch">
          <button @click="toggleLanguageMenu" class="header-link" :title="currentLangOption?.label">
            <span class="lang-flag-icon">{{ currentLangOption?.flag }}</span>
            <span class="link-text">{{ currentLangOption?.shortLabel }}</span>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"
              stroke-linecap="round">
              <polyline points="6 9 12 15 18 9" />
            </svg>
          </button>

          <div v-if="showLanguageMenu" class="language-dropdown">
            <div v-for="lang in languageOptions" :key="lang.value" @click="selectLanguage(lang.value)"
              class="language-option" :class="{ active: currentLanguage === lang.value }">
              <span class="lang-flag">{{ lang.flag }}</span>
              <span class="lang-label">{{ lang.label }}</span>
              <span v-if="currentLanguage === lang.value" class="check-icon">✓</span>
            </div>
          </div>
        </div>
      </div>
    </header>

    <section class="showcase-section">
      <div class="showcase-content">
        <div class="showcase-overline"><span></span>KNOWLEDGE SPACE ONLINE</div>
        <h1 class="showcase-title">{{ $t('platform.subtitle') }}</h1>
        <p class="showcase-description">{{ $t('platform.description') }}</p>
        <div class="feature-tags">
          <span class="tag">{{ $t('platform.rag') }}</span>
          <span class="tag">{{ $t('platform.agent') }}</span>
          <span class="tag">{{ $t('platform.wiki') }}</span>
          <span class="tag">{{ $t('platform.hybridSearch') }}</span>
        </div>
        <div class="graph-meta">
          <span class="graph-meta__pulse"></span>
          <strong>KY-01 / KNOWLEDGE GRAPH</strong>
          <span>INDEX SYNCHRONIZED</span>
          <span class="graph-meta__hint">DRAG · ZOOM · EXPLORE</span>
        </div>
      </div>
    </section>

    <!-- Right Form Section -->
    <div class="form-section">
      <div class="form-panel">
        <!-- Login Card -->
        <div class="form-card" v-if="!isRegisterMode">
          <div class="form-header">
            <h2 class="form-title">{{ $t('auth.login') }}</h2>
            <p class="form-welcome">{{ $t('auth.subtitle') }}</p>
            <p v-if="registrationEnabled" class="form-hint">{{ $t('auth.loginHint') }}</p>
          </div>

          <div class="form-content">
            <t-form ref="formRef" :data="formData" :rules="formRules" @submit="handleLogin" layout="vertical">
              <t-form-item :label="$t('auth.email')" name="email">
                <t-input v-model="formData.email" :placeholder="$t('auth.emailPlaceholder')" type="text"
                  autocomplete="email" size="large" :disabled="loading" />
              </t-form-item>

              <t-form-item :label="$t('auth.password')" name="password">
                <t-input v-model="formData.password" :placeholder="$t('auth.passwordPlaceholder')" type="password"
                  autocomplete="current-password" size="large" :disabled="loading" @enter="handleLogin" />
              </t-form-item>

              <t-button type="submit" theme="primary" size="large" block :loading="loading" class="submit-button">
                {{ loading ? $t('auth.loggingIn') : $t('auth.login') }}
              </t-button>

              <div class="register-cta" v-if="registrationEnabled">
                <div class="register-cta__divider">
                  <span>{{ $t('auth.firstTime') }}</span>
                </div>
                <t-button theme="default" variant="outline" size="large" block class="register-cta__button"
                  :disabled="loading" @click="toggleMode">
                  {{ $t('auth.createAccount') }}
                </t-button>
              </div>

              <div v-if="oidcEnabled" class="oidc-divider">
                <span>{{ $t('auth.orContinueWith') }}</span>
              </div>

              <t-button v-if="oidcEnabled" theme="default" size="large" block :loading="oidcLoading" :disabled="loading"
                class="oidc-button" @click="handleOIDCLogin">
                {{ oidcLoading ? $t('auth.redirectingToOIDC') : oidcLoginText }}
              </t-button>
            </t-form>

            <!-- Features list -->
            <div class="login-features">
              <div class="feature-item">
                <span class="feature-icon">✓</span>
                <span class="feature-text">{{ $t('platform.multimodalParsing') }}</span>
              </div>
              <div class="feature-item">
                <span class="feature-icon">✓</span>
                <span class="feature-text">{{ $t('platform.hybridSearchEngine') }}</span>
              </div>
              <div class="feature-item">
                <span class="feature-icon">✓</span>
                <span class="feature-text">{{ $t('platform.ragQandA') }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Register Card. Renders when the user is in register mode
             AND either self-service registration is enabled OR they
             arrived with a valid share-link token (which bypasses the
             invite_only gate). -->
        <div class="form-card" v-if="isRegisterMode && (registrationEnabled || inviteLookup)">
          <!-- Share-link banner: shown only when ?token= resolved to a
               real invitation row. Sits above the form header so the
               invitee instantly sees who invited them and into which
               workspace, without bumping the existing register UX. -->
          <div v-if="inviteLookup" class="invite-banner">
            <t-icon name="link" class="invite-banner__icon" />
            <div class="invite-banner__text">
              <div class="invite-banner__title">
                {{ $t('inviteRegister.bannerTitle', { tenant: inviteLookup.tenant_name || '' }) }}
              </div>
              <div class="invite-banner__hint">
                {{ $t('inviteRegister.bannerHint') }}
              </div>
            </div>
          </div>
          <div v-else-if="inviteLookupError" class="invite-banner invite-banner--error">
            {{ inviteLookupError }}
          </div>
          <div class="form-header">
            <h2 class="form-title">{{ $t('auth.createAccount') }}</h2>
            <p class="form-subtitle">{{ $t('auth.registerSubtitle') }}</p>
          </div>

          <div class="form-content">
            <t-form ref="registerFormRef" :data="registerData" :rules="registerRules" @submit="handleRegister"
              layout="vertical">
              <t-form-item :label="$t('auth.username')" name="username">
                <t-input v-model="registerData.username" :placeholder="$t('auth.usernamePlaceholder')" size="large"
                  :disabled="loading" />
              </t-form-item>

              <t-form-item :label="$t('auth.email')" name="email">
                <t-input v-model="registerData.email" :placeholder="$t('auth.emailPlaceholder')" type="text"
                  autocomplete="email" size="large" :disabled="loading" />
              </t-form-item>

              <t-form-item :label="$t('auth.password')" name="password">
                <t-input v-model="registerData.password" :placeholder="$t('auth.passwordPlaceholder')" type="password"
                  autocomplete="new-password" size="large" :disabled="loading" />
              </t-form-item>

              <t-form-item :label="$t('auth.confirmPassword')" name="confirmPassword">
                <t-input v-model="registerData.confirmPassword" :placeholder="$t('auth.confirmPasswordPlaceholder')"
                  type="password" autocomplete="new-password" size="large" :disabled="loading" @enter="handleRegister" />
              </t-form-item>

              <t-button type="submit" theme="primary" size="large" block :loading="loading" class="submit-button">
                {{ loading ? $t('auth.registering') : $t('auth.register') }}
              </t-button>
            </t-form>

            <div class="form-footer">
              <span>{{ $t('auth.haveAccount') }}</span>
              <a href="#" @click.prevent="toggleMode" class="link-button">
                {{ $t('auth.backToLogin') }}
              </a>
            </div>

            <!-- Features list for register -->
            <div class="login-features">
              <div class="feature-item">
                <span class="feature-icon">✓</span>
                <span class="feature-text">{{ $t('platform.independentTenant') }}</span>
              </div>
              <div class="feature-item">
                <span class="feature-icon">✓</span>
                <span class="feature-text">{{ $t('platform.fullApiAccess') }}</span>
              </div>
              <div class="feature-item">
                <span class="feature-icon">✓</span>
                <span class="feature-text">{{ $t('platform.knowledgeBaseManagement') }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
    <footer class="auth-footer">FTMind · Intelligent Knowledge System</footer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, nextTick, onMounted, onBeforeUnmount, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { useRoleLabel } from '@/composables/useRoleLabel'
import { notifyLoginSuccess } from '@/utils/loginNotify'
import brandLogo from '@/assets/img/brand/sf-logo-alone.png'
import {
  login,
  register,
  getOIDCAuthorizationURL,
  getOIDCConfig,
  autoSetup,
  getAuthConfig,
  userInfoFromApi,
  getInvitationByToken,
  registerByInvite,
  type InviteLookup,
} from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { t, tm, locale } = useI18n()
const { formatRole, roleIcon } = useRoleLabel()

// Form references
const formRef = ref()
const registerFormRef = ref()

// State management
const loading = ref(false)
const oidcLoading = ref(false)
const isRegisterMode = ref(false)
const showLanguageMenu = ref(false)
const oidcEnabled = ref(false)
const oidcProviderName = ref('')
// registrationEnabled defaults to true so that on first paint the Register
// link is visible; the actual mode is fetched from /auth/config in onMounted.
// In invite_only mode the link/card are hidden.
const registrationEnabled = ref(true)

// invite-link state. When the URL carries ?token=xxx we resolve it to
// the originating tenant + role and switch the form into a "register
// via invitation" mode. The token bypasses the normal invite_only
// gate — possessing it IS the authorisation. Submitting the register
// form with this set hits /auth/register-by-invite (auto-login on
// success) instead of /auth/register.
const inviteToken = ref('')
const inviteLookup = ref<InviteLookup | null>(null)
const inviteLookupError = ref('')
const inviteLookupLoading = ref(false)

// Language options
const languageOptions = [
  { value: 'zh-CN', label: '简体中文', shortLabel: '中文', flag: '🇨🇳' },
  { value: 'en-US', label: 'English', shortLabel: 'EN', flag: '🇺🇸' },
  { value: 'ru-RU', label: 'Русский', shortLabel: 'RU', flag: '🇷🇺' },
  { value: 'ko-KR', label: '한국어', shortLabel: '한국어', flag: '🇰🇷' }
]

const currentLanguage = computed(() => locale.value)
const oidcLoginText = computed(() => {
  if (oidcProviderName.value) {
    return t('auth.oidcLoginWithProvider', { provider: oidcProviderName.value })
  }
  return t('auth.oidcLogin')
})
const currentLangOption = computed(() => languageOptions.find(l => l.value === currentLanguage.value))

// Login form data
const formData = reactive<{ [key: string]: any }>({
  email: '',
  password: '',
})

// Register form data
const registerData = reactive<{ [key: string]: any }>({
  username: '',
  email: '',
  password: '',
  confirmPassword: ''
})

// Login form validation rules
const formRules = computed(() => ({
  email: [
    { required: true, message: t('auth.emailRequired'), type: 'error' },
    { email: true, message: t('auth.emailInvalid'), type: 'error' }
  ],
  password: [
    { required: true, message: t('auth.passwordRequired'), type: 'error' },
    { min: 8, message: t('auth.passwordMinLength'), type: 'error' },
    { max: 32, message: t('auth.passwordMaxLength'), type: 'error' },
    { pattern: /[a-zA-Z]/, message: t('auth.passwordMustContainLetter'), type: 'error' },
    { pattern: /\d/, message: t('auth.passwordMustContainNumber'), type: 'error' }
  ]
}))

// Register form validation rules
const registerRules = computed(() => ({
  username: [
    { required: true, message: t('auth.usernameRequired'), type: 'error' },
    { min: 2, message: t('auth.usernameMinLength'), type: 'error' },
    { max: 20, message: t('auth.usernameMaxLength'), type: 'error' },
    {
      pattern: /^[a-zA-Z0-9_\u4e00-\u9fa5]+$/,
      message: t('auth.usernameInvalid'),
      type: 'error'
    }
  ],
  email: [
    { required: true, message: t('auth.emailRequired'), type: 'error' },
    { email: true, message: t('auth.emailInvalid'), type: 'error' }
  ],
  password: [
    { required: true, message: t('auth.passwordRequired'), type: 'error' },
    { min: 8, message: t('auth.passwordMinLength'), type: 'error' },
    { max: 32, message: t('auth.passwordMaxLength'), type: 'error' },
    { pattern: /[a-zA-Z]/, message: t('auth.passwordMustContainLetter'), type: 'error' },
    { pattern: /\d/, message: t('auth.passwordMustContainNumber'), type: 'error' }
  ],
  confirmPassword: [
    { required: true, message: t('auth.confirmPasswordRequired'), type: 'error' },
    {
      validator: (val: string) => val === registerData.password,
      message: t('auth.passwordMismatch'),
      type: 'error'
    }
  ]
}))

// Toggle login/register mode
const toggleMode = () => {
  isRegisterMode.value = !isRegisterMode.value

  Object.keys(registerData).forEach(key => {
    (registerData as any)[key] = ''
  })
}

// Toggle language menu
const toggleLanguageMenu = () => {
  showLanguageMenu.value = !showLanguageMenu.value
}

// Select language
const selectLanguage = (lang: string) => {
  locale.value = lang
  localStorage.setItem('locale', lang)
  showLanguageMenu.value = false
  MessagePlugin.success(t('language.languageSaved'))
}

// Close language menu when clicking outside
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (!target.closest('.language-switch')) {
    showLanguageMenu.value = false
  }
}

// Add click outside listener
onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})

const persistLoginResponse = async (response: any) => {
  // Backend renamed `tenant` to `active_tenant` and added `memberships`
  // when tenant-level RBAC landed (issue #1303). The two are otherwise
  // identical — `active_tenant` is the tenant whose ID is encoded in the
  // JWT, defaulting to the user's home tenant on a fresh login.
  const activeTenant = response.active_tenant || response.tenant
  if (response.user && response.token) {
    // user.tenant_id must be the user's HOME tenant (the immutable row
    // on the users table); useHomeTenant() and the home-badge logic both
    // assume so. The ACTIVE tenant (which can differ from home when the
    // server honoured a remembered last-active-tenant preference) is
    // expressed separately via setSelectedTenant below.
    const homeTenantIdRaw = response.user.tenant_id ?? activeTenant?.id ?? ''
    authStore.setUser(userInfoFromApi(response.user, homeTenantIdRaw))
    authStore.setToken(response.token)
    if (response.refresh_token) {
      authStore.setRefreshToken(response.refresh_token)
    }
    if (activeTenant) {
      authStore.setTenant({
        id: String(activeTenant.id) || '',
        name: activeTenant.name || '',
        owner_id: response.user.id || '',
        created_at: activeTenant.created_at || new Date().toISOString(),
        updated_at: activeTenant.updated_at || new Date().toISOString()
      })
    } else {
      authStore.setTenant(null)
    }
    if (Array.isArray(response.memberships)) {
      authStore.setMemberships(response.memberships)
    }
    // If the backend dropped us into a non-home tenant (honoured a
    // remembered "last active tenant" preference), set the override so
    // subsequent requests carry X-Tenant-ID and the UI stays consistent.
    // Otherwise clear any stale override left in localStorage by a
    // previous session for a different account.
    const activeIdNum = Number(activeTenant?.id)
    const homeIdNum = Number(homeTenantIdRaw)
    if (Number.isFinite(activeIdNum) && Number.isFinite(homeIdNum) && activeIdNum !== homeIdNum) {
      authStore.setSelectedTenant(activeIdNum, activeTenant?.name || null)
    } else {
      authStore.setSelectedTenant(null, null)
    }
  }

  // Pull runtime capabilities (including whether ordinary users may create
  // workspaces) before entering the main UI so create actions never flash
  // briefly when the deployment is invitation-only.
  await authStore.refreshFromAuthMe()
  await nextTick()
  router.replace(authStore.hasValidTenant ? '/platform/knowledge-bases' : '/onboarding/workspace')
}

const getBackendOIDCRedirectURI = () => `${window.location.origin}/api/v1/auth/oidc/callback`

const loadOIDCConfig = async () => {
  try {
    const response = await getOIDCConfig()
    oidcEnabled.value = !!response.success && !!response.enabled
    oidcProviderName.value = response.provider_display_name || ''
  } catch {
    oidcEnabled.value = false
    oidcProviderName.value = ''
  }
}

// loadAuthConfig fetches /auth/config and caches whether self-service
// registration is allowed. Failures fall back to "enabled" so a transient
// network glitch doesn't lock new users out of an open deployment.
const loadAuthConfig = async () => {
  try {
    const response = await getAuthConfig()
    registrationEnabled.value = response.registration_mode !== 'invite_only'
  } catch {
    registrationEnabled.value = true
  }
}

const handleOIDCLogin = async () => {
  try {
    oidcLoading.value = true
    const response = await getOIDCAuthorizationURL(getBackendOIDCRedirectURI())
    const authorizationURL = response.authorization_url

    if (!response.success || !authorizationURL) {
      MessagePlugin.error(response.message || t('auth.oidcLoginFailed'))
      return
    }

    window.location.href = authorizationURL
  } catch (error: any) {
    console.error('OIDC 登录跳转失败:', error)
    MessagePlugin.error(error.message || t('auth.oidcLoginFailed'))
  } finally {
    oidcLoading.value = false
  }
}

// Handle login
const handleLogin = async () => {
  try {
    const valid = await formRef.value?.validate()
    if (valid !== true) return

    loading.value = true

    const response = await login({
      email: formData.email,
      password: formData.password,
    })

    if (response.success) {
      await persistLoginResponse(response)
      notifyLoginSuccess(response, t, tm, formatRole, roleIcon)
    } else {
      MessagePlugin.error(response.message || t('auth.loginError'))
    }
  } catch (error: any) {
    console.error('登录错误:', error)
    MessagePlugin.error(error.message || t('auth.loginErrorRetry'))
  } finally {
    loading.value = false
  }
}

// Handle registration. Dispatches based on whether the user arrived
// with a share-link token: with token -> register-by-invite (auto-
// login on success); without -> the normal self-service register
// (drops back to the login form for the user to sign in).
const handleRegister = async () => {
  try {
    const valid = await registerFormRef.value?.validate()
    if (valid !== true) return

    loading.value = true

    if (inviteToken.value) {
      const response = await registerByInvite({
        token: inviteToken.value,
        username: registerData.username,
        email: registerData.email,
        password: registerData.password,
      })
      if (!response.success) {
        MessagePlugin.error(response.message || t('auth.registerFailed'))
        return
      }
      MessagePlugin.success(t('auth.registerSuccess'))
      // register-by-invite returns the same shape as login (token +
      // active_tenant + memberships), so reuse the login persistence
      // path — same store writes, same redirect target.
      await persistLoginResponse(response)
      return
    }

    const response = await register({
      username: registerData.username,
      email: registerData.email,
      password: registerData.password
    })

    if (response.success) {
      MessagePlugin.success(t('auth.registerSuccess'))

      // Switch to login mode and fill in email
      isRegisterMode.value = false
      formData.email = registerData.email

      // Clear register form
      Object.keys(registerData).forEach(key => {
        (registerData as any)[key] = ''
      })
    } else {
      MessagePlugin.error(response.message || t('auth.registerFailed'))
    }
  } catch (error: any) {
    console.error('注册错误:', error)
    MessagePlugin.error(error.message || t('auth.registerError'))
  } finally {
    loading.value = false
  }
}

// Check if already logged in; for lite edition, attempt transparent auto-setup
onMounted(async () => {
  // Share-link landing: ?token=xxx switches the form into invite-
  // register mode before any other auto-flow (logged-in redirect /
  // auto-setup / OIDC) gets a chance to redirect. Resolution failure
  // surfaces inline; the user can still log in normally if they
  // already have an account. We check this BEFORE the isLoggedIn
  // redirect so an existing session doesn't bounce the user to
  // /platform (and possibly back to /login if the session is stale),
  // dropping the invite token along the way.
  const tokenFromQuery = String(route.query.token || '').trim()
  if (tokenFromQuery) {
    inviteToken.value = tokenFromQuery
    inviteLookupLoading.value = true
    try {
      const resp = await getInvitationByToken(tokenFromQuery)
      if (resp.success && resp.data) {
        inviteLookup.value = resp.data
        // Token bypasses invite_only — show the register card even
        // when self-service registration is otherwise disabled.
        registrationEnabled.value = true
        isRegisterMode.value = true
      } else {
        inviteLookupError.value = resp.message || t('inviteRegister.invalidBody')
      }
    } catch {
      inviteLookupError.value = t('inviteRegister.invalidBody')
    } finally {
      inviteLookupLoading.value = false
    }
    // Don't run auto-setup when the user came in via an invite link —
    // they're explicitly trying to register, not bootstrap a Lite
    // single-user instance.
    loadOIDCConfig()
    return
  }

  // Do not trust a stale in-memory Pinia token after the request interceptor
  // has invalidated the persisted session. Otherwise /login immediately
  // redirects back to the protected shell and the router can visibly bounce.
  if (authStore.isLoggedIn && localStorage.getItem('fmind_token')) {
    router.replace('/platform/knowledge-bases')
    return
  }

  const AUTO_SETUP_FAILED_KEY = 'fmind_auto_setup_failed'
  if (localStorage.getItem(AUTO_SETUP_FAILED_KEY) !== 'true') {
    try {
      const response = await autoSetup()
      if (response.success) {
        authStore.setLiteMode(true)
        await persistLoginResponse(response)
        return
      } else {
        localStorage.setItem(AUTO_SETUP_FAILED_KEY, 'true')
      }
    } catch {
      localStorage.setItem(AUTO_SETUP_FAILED_KEY, 'true')
    }
  }

  loadOIDCConfig()
  loadAuthConfig()
})
</script>

<style lang="less" scoped>
.login-layout {
  display: flex;
  width: 100%;
  min-height: 100%;
  overflow: hidden;
  position: relative;
  background: linear-gradient(225deg, #111827 0%, #1e1b4b 15%, #312e81 25%, #3730a3 38%, #4f46e5 50%, #5b5bd6 65%, #3b82f6 78%, #8b5cf6 90%, #c4b5fd 100%);

  &::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: radial-gradient(circle at 20% 50%, rgba(255, 255, 255, 0.06) 0%, transparent 50%),
      radial-gradient(circle at 80% 50%, rgba(255, 255, 255, 0.04) 0%, transparent 50%);
    pointer-events: none;
  }
}

.animated-bg {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  pointer-events: none;
  z-index: 1;
  overflow: hidden;
  contain: strict;
}

.knowledge-node {
  position: absolute;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.15);
  border: 2px solid rgba(255, 255, 255, 0.3);
  box-shadow:
    0 0 15px rgba(255, 255, 255, 0.35),
    0 0 30px rgba(91, 91, 214, 0.24),
    inset 0 0 8px rgba(255, 255, 255, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  animation: nodePulse 5s infinite ease-in-out;
  will-change: transform, opacity;
}

.node-icon {
  width: 20px;
  height: 20px;
  color: rgba(255, 255, 255, 0.9);
}

.node-1 {
  top: 15%;
  left: 20%;
  animation-delay: 0s;
}

.node-2 {
  top: 25%;
  left: 35%;
  animation-delay: 0.5s;
}

.node-3 {
  top: 20%;
  left: 55%;
  animation-delay: 1s;
}

.node-4 {
  top: 30%;
  left: 75%;
  animation-delay: 1.5s;
}

.node-5 {
  top: 45%;
  left: 25%;
  animation-delay: 2s;
}

.node-6 {
  top: 50%;
  left: 45%;
  animation-delay: 2.5s;
}

.node-7 {
  top: 48%;
  left: 65%;
  animation-delay: 3s;
}

.node-8 {
  top: 60%;
  left: 20%;
  animation-delay: 0.3s;
}

.node-9 {
  top: 12%;
  right: 15%;
  animation-delay: 1.8s;
}

.node-10 {
  top: 38%;
  right: 10%;
  animation-delay: 2.3s;
}

.node-11 {
  top: 70%;
  left: 40%;
  animation-delay: 0.8s;
}

.node-12 {
  top: 65%;
  left: 80%;
  animation-delay: 1.3s;
}

@keyframes nodePulse {

  0%,
  100% {
    transform: scale(1);
    opacity: 0.65;
  }

  50% {
    transform: scale(1.08);
    opacity: 0.9;
  }
}

.knowledge-lines {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  opacity: 0.35;
}

.connection-line {
  stroke: rgba(255, 255, 255, 0.5);
  stroke-width: 1.5;
  stroke-dasharray: 6, 3;
  stroke-linecap: round;
  animation: lineFlow 10s infinite linear;
  will-change: stroke-dashoffset;
}

.line-1 {
  animation-delay: 0s;
}

.line-2 {
  animation-delay: 0.5s;
}

.line-3 {
  animation-delay: 1s;
}

.line-4 {
  animation-delay: 0.3s;
}

.line-5 {
  animation-delay: 0.8s;
}

.line-6 {
  animation-delay: 1.3s;
}

.line-7 {
  animation-delay: 1.8s;
}

.line-8 {
  animation-delay: 2.3s;
}

.line-9 {
  animation-delay: 0.2s;
}

.line-10 {
  animation-delay: 0.7s;
}

.line-11 {
  animation-delay: 0.9s;
}

.line-12 {
  animation-delay: 1.5s;
}

@keyframes lineFlow {
  0% {
    stroke-dashoffset: 0;
  }

  100% {
    stroke-dashoffset: 18;
  }
}

@media (prefers-reduced-motion: reduce) {
  .knowledge-node {
    animation: none;
    opacity: 0.65;
  }

  .connection-line {
    animation: none;
  }
}

/* Left Showcase Section */
.showcase-section {
  flex: 0 0 52%;
  display: flex;
  align-items: flex-end;
  padding: 100px 30px 100px 50px;
  box-sizing: border-box;
  position: relative;
}

.showcase-content {
  width: 100%;
  max-width: 600px;
  position: relative;
  z-index: 2;
  display: flex;
  flex-direction: column;
  margin-bottom: 60px;
}

.showcase-subtitle {
  margin-top: 0;
  font-size: 22px;
  color: rgba(255, 255, 255, 0.95);
  margin: 0 0 8px 0;
  font-family: var(--app-font-family);
  line-height: 1.4;
  font-weight: 500;
}

.showcase-description {
  font-size: 15px;
  color: rgba(255, 255, 255, 0.8);
  margin: 0 0 28px 0;
  font-family: var(--app-font-family);
  line-height: 1.5;
}

.feature-tags {
  display: flex;
  gap: 12px;
  margin-bottom: 40px;
  flex-wrap: wrap;
}

.tag {
  display: inline-block;
  padding: 8px 20px;
  background: rgba(255, 255, 255, 0.2);
  border-radius: 20px;
  color: var(--td-text-color-anti);
  font-size: 14px;
  font-weight: 500;
  font-family: var(--app-font-family);
}

/* Carousel */
.carousel-container {
  width: 100%;
  margin-top: 48px;
}

.screenshot-swiper {
  width: 100%;
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  padding-bottom: 40px;

  :deep(.swiper-wrapper) {
    transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
  }

  :deep(.swiper-pagination) {
    bottom: 15px !important;
    z-index: 10;
  }

  :deep(.swiper-pagination-bullet) {
    width: 10px;
    height: 10px;
    background: rgba(255, 255, 255, 0.5);
    opacity: 1;
    transition: all 0.3s ease;
    margin: 0 6px !important;
  }

  :deep(.swiper-pagination-bullet-active) {
    background: var(--td-bg-color-container);
    width: 28px;
    border-radius: 5px;
  }
}

.slide-content {
  width: 100%;
  height: 100%;
  background: var(--td-bg-color-container);
  border-radius: 16px;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}

.slide-image {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: contain;
}

/* Right Form Section */
.form-section {
  flex: 0 0 48%;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  padding: 40px 50px 100px 30px;
  box-sizing: border-box;
  position: relative;
}

.form-panel {
  width: 100%;
  max-width: 480px;
  margin-bottom: 60px;
  position: relative;
  z-index: 2;
}

.header-logo {
  position: fixed;
  top: 32px;
  left: 50px;
  z-index: 100;
  cursor: pointer;

  .logo-image {
    width: 120px;
    height: auto;
  }
}

.header-links {
  position: fixed;
  top: 28px;
  right: 28px;
  display: flex;
  align-items: center;
  gap: 10px;
  z-index: 100;
}

.header-link {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 9px 15px;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.25);
  color: var(--td-text-color-anti);
  text-decoration: none;
  font-size: 13px;
  font-weight: 600;
  font-family: var(--app-font-family);
  letter-spacing: 0.2px;
  cursor: pointer;
  position: relative;

  svg {
    flex-shrink: 0;
  }

  .link-text {
    line-height: 1;
  }

  &:hover {
    background: rgba(255, 255, 255, 0.3);
    border-color: rgba(255, 255, 255, 0.4);
    color: var(--td-text-color-anti);
  }
}

.language-switch {
  position: relative;

  button {
    background: rgba(255, 255, 255, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.25);
    color: var(--td-text-color-anti);

    .lang-flag-icon {
      font-size: 16px;
      line-height: 1;
      flex-shrink: 0;
    }

    &:hover {
      background: rgba(255, 255, 255, 0.3);
      border-color: rgba(255, 255, 255, 0.4);
    }

    svg:last-child {
      margin-left: 2px;
      flex-shrink: 0;
    }
  }
}

.language-dropdown {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  min-width: 160px;
  background: rgba(255, 255, 255, 0.97);
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12);
  overflow: hidden;
  z-index: 1000;
}

.language-option {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  cursor: pointer;
  font-size: 13px;
  font-family: var(--app-font-family);
  color: var(--td-text-color-primary);

  .lang-flag {
    font-size: 16px;
    flex-shrink: 0;
  }

  .lang-label {
    flex: 1;
  }

  .check-icon {
    color: var(--td-success-color);
    font-weight: 700;
    font-size: 14px;
    flex-shrink: 0;
  }

  &:hover {
    background: var(--td-bg-color-secondarycontainer);
  }

  &.active {
    background: var(--td-success-color-light);
    color: var(--td-brand-color-active);
  }
}

.form-card {
  background: rgba(255, 255, 255, 0.97);
  border-radius: 16px;
  padding: 40px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.15);
  box-sizing: border-box;
  border: none;
  width: 100%;
}

/* Share-link invitation banner. Sits above the register form when the
 * user arrived via /register?token=xxx; gives them confirmation of who
 * invited them before they fill anything in. Subtle, neutral card —
 * the page background is heavily brand-coloured already, so a loud
 * tinted banner clashes; we lean on the form's own surface tokens. */
.invite-banner {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  margin-bottom: 20px;
  border-radius: 10px;
  background: var(--td-bg-color-container-hover, rgba(0, 0, 0, 0.03));
  border: 1px solid var(--td-component-stroke);
  color: var(--td-text-color-primary);
}

.invite-banner__icon {
  margin-top: 2px;
  font-size: 18px;
  flex-shrink: 0;
  color: var(--td-text-color-secondary);
}

.invite-banner__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.invite-banner__title {
  font-size: 14px;
  font-weight: 600;
  line-height: 1.4;
  color: var(--td-text-color-primary);
}

.invite-banner__hint {
  font-size: 12px;
  color: var(--td-text-color-secondary);
  line-height: 1.5;
}

.invite-banner--error {
  background: var(--td-error-color-1, rgba(220, 38, 38, 0.06));
  border-color: var(--td-error-color-3, rgba(220, 38, 38, 0.2));
  color: var(--td-error-color, #b91c1c);
  font-size: 13px;
}

.form-header {
  text-align: center;
  margin-bottom: 32px;
}

.form-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin: 0 0 6px 0;
  font-family: var(--app-font-family);
}

.form-welcome {
  font-size: 13px;
  color: var(--td-text-color-secondary);
  margin: 0;
  font-family: var(--app-font-family);
}

.form-hint {
  margin: 10px 0 0;
  padding: 8px 12px;
  border-radius: 8px;
  background: var(--td-brand-color-light, rgba(91, 91, 214, 0.08));
  color: var(--td-brand-color-active);
  font-size: 12.5px;
  line-height: 1.5;
  font-family: var(--app-font-family);
}

/* 注册入口：从底部小字链接升级为带分隔线的醒目次级按钮，
   让首次访客一眼就能找到「创建账户」。 */
.register-cta {
  margin-top: 8px;

  &__divider {
    position: relative;
    text-align: center;
    margin: 4px 0 14px;
    color: var(--td-text-color-secondary);
    font-size: 13px;
    font-family: var(--app-font-family);

    span {
      position: relative;
      z-index: 1;
      padding: 0 12px;
      background: rgba(255, 255, 255, 0.97);
    }

    &::before {
      content: '';
      position: absolute;
      left: 0;
      right: 0;
      top: 50%;
      border-top: 1px solid var(--td-component-stroke);
    }
  }

  &__button {
    height: 46px;
    border-radius: 8px;
    font-size: 15px;
    font-weight: 500;
    border-color: var(--td-brand-color);
    color: var(--td-brand-color);

    &:hover {
      border-color: var(--td-brand-color-active);
      color: var(--td-brand-color-active);
      background: var(--td-brand-color-light, rgba(91, 91, 214, 0.08));
    }
  }
}

.form-subtitle {
  font-size: 13px;
  color: var(--td-text-color-secondary);
  margin: 0;
  font-family: var(--app-font-family);
}

.form-content {
  :deep(.t-form-item__label) {
    font-size: 14px;
    color: var(--td-text-color-primary);
    font-weight: 500;
    margin-bottom: 8px;
    font-family: var(--app-font-family);
    display: block;
    text-align: left;
  }

  :deep(.t-input) {
    border: 1px solid var(--td-component-stroke);
    border-radius: 8px;
    background: var(--td-bg-color-container);
    transition: all 0.2s;

    &:focus-within {
      border-color: var(--td-brand-color);
      box-shadow: 0 0 0 3px rgba(91, 91, 214, 0.12);
    }

    &:hover {
      border-color: var(--td-brand-color);
    }

    .t-input__inner {
      border: none !important;
      box-shadow: none !important;
      outline: none !important;
      background: transparent;
      font-size: 15px;
      font-family: var(--app-font-family);

      &:focus {
        border: none !important;
        box-shadow: none !important;
        outline: none !important;
      }
    }

    .t-input__wrap {
      border: none !important;
      box-shadow: none !important;
    }
  }

  :deep(.t-form-item) {
    margin-bottom: 18px;

    &:last-child {
      margin-bottom: 0;
    }
  }

  :deep(.t-form-item__control) {
    width: 100%;
  }
}

.submit-button {
  height: 46px;
  border-radius: 8px;
  font-size: 16px;
  font-weight: 500;
  font-family: var(--app-font-family);
  margin: 20px 0 16px 0;
}

.oidc-divider {
  position: relative;
  margin: 4px 0 6px;
  text-align: center;
  color: var(--td-text-color-placeholder);
  font-size: 12px;

  span {
    position: relative;
    z-index: 1;
    padding: 0 12px;
    background: rgba(255, 255, 255, 0.95);
  }

  &::before {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    top: 50%;
    border-top: 1px solid var(--td-component-stroke);
  }
}

.oidc-button {
  height: 46px;
  border-radius: 8px;
  font-size: 15px;
  font-weight: 500;
}

.form-footer {
  text-align: center;
  font-size: 14px;
  color: var(--td-text-color-secondary);
  font-family: var(--app-font-family);
  margin-top: 16px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--td-component-stroke);

  .link-button {
    color: var(--td-brand-color);
    text-decoration: none;
    margin-left: 4px;
    font-weight: 500;
    transition: all 0.2s;

    &:hover {
      color: var(--td-brand-color);
      text-decoration: underline;
    }
  }
}

.login-form-footer {
  border-bottom: none;
  padding-bottom: 8px;
  margin-top: 12px;
}

.login-features {
  margin-top: 20px;
  padding: 0;

  .feature-item {
    display: flex;
    align-items: center;
    margin-bottom: 12px;
    font-size: 13px;
    color: var(--td-text-color-secondary);
    font-family: var(--app-font-family);

    &:last-child {
      margin-bottom: 0;
    }

    .feature-icon {
      width: 20px;
      height: 20px;
      border-radius: 50%;
      background: var(--td-success-color-light);
      color: var(--td-brand-color-active);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 12px;
      font-weight: 700;
      margin-right: 10px;
      flex-shrink: 0;
    }

    .feature-text {
      line-height: 1.4;
    }
  }
}

/* Responsive Design */
@media (max-width: 1024px) {
  .knowledge-node:nth-of-type(n + 13) {
    display: none;
  }

  .connection-line:nth-of-type(n + 13) {
    display: none;
  }

  .showcase-subtitle {
    font-size: 18px;
  }

  .header-logo {
    top: 26px;
    left: 40px;

    .logo-image {
      width: 100px;
    }
  }

  .header-links {
    top: 22px;
    right: 22px;
    gap: 8px;

    .link-text {
      display: none;
    }

    .header-link {
      padding: 10px;
      gap: 0;
    }
  }
}

@media (max-width: 768px) {
  .login-layout {
    flex-direction: column;
  }

  .knowledge-node:nth-of-type(n + 9) {
    display: none;
  }

  .connection-line:nth-of-type(n + 9) {
    display: none;
  }

  .showcase-section {
    flex: 0 0 auto;
    min-height: 50vh;
    padding: 40px 24px;
  }

  .showcase-content {
    max-width: 100%;
  }

  .header-logo {
    top: 22px;
    left: 30px;

    .logo-image {
      width: 80px;
    }
  }

  .showcase-subtitle {
    font-size: 16px;
    margin-bottom: 24px;
  }

  .feature-tags {
    margin-bottom: 24px;
  }

  .carousel-container {
    margin-top: 24px;
  }

  .form-section {
    flex: 0 0 auto;
    padding: 24px;
  }

  .header-links {
    top: 18px;
    right: 18px;
    gap: 8px;

    .link-text {
      display: inline;
    }

    .header-link {
      padding: 8px 12px;
      font-size: 12px;
    }
  }

  .form-card {
    padding: 32px 24px;
  }

  .form-title {
    font-size: 22px;
  }
}

@media (max-width: 480px) {
  .animated-bg {
    display: none;
  }

  .showcase-section {
    padding: 32px 20px;
  }

  .header-logo {
    top: 18px;
    left: 20px;

    .logo-image {
      width: 70px;
    }
  }

  .showcase-subtitle {
    font-size: 14px;
  }

  .tag {
    font-size: 12px;
    padding: 6px 16px;
  }

  .form-section {
    padding: 20px;
  }

  .header-links {
    top: 14px;
    right: 14px;
    gap: 6px;
    flex-wrap: wrap;

    .header-link {
      padding: 7px 10px;
      font-size: 11px;
    }
  }

  .form-card {
    padding: 28px 20px;
  }

  .form-header {
    margin-bottom: 24px;
  }
}

@media (prefers-reduced-motion: reduce) {

  .knowledge-node,
  .connection-line {
    animation: none !important;
    transition: none !important;
  }

  .animated-bg {
    display: none;
  }
}
</style>

<style lang="less">
/* Final login-only overrides: keep the new composition above legacy scoped rules. */
.login-layout.login-layout--gateway {
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
  min-height: 100vh !important;
  padding: 96px 24px 72px !important;
  background: #f4f8ff !important;
}
.login-layout.login-layout--gateway::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 44vh;
  background: linear-gradient(135deg, #003cab, #0052d9 58%, #1677ff);
  clip-path: polygon(0 0, 100% 0, 100% 72%, 78% 88%, 48% 76%, 19% 91%, 0 78%);
}
.login-layout .auth-topbar {
  position: absolute !important;
  inset: 0 0 auto !important;
  height: 84px !important;
  padding: 0 42px !important;
  z-index: 5 !important;
}
.login-layout .header-logo {
  position: static !important;
  display: inline-flex !important;
  align-items: center;
  gap: 10px;
  color: #fff !important;
}
.login-layout .header-logo .logo-image {
  width: 34px !important;
  height: 34px !important;
  object-fit: contain;
  filter: none !important;
}
.login-layout .header-logo .logo-wordmark {
  color: #fff !important;
  font-size: 22px;
  font-weight: 750;
}
.login-layout .showcase-section,
.login-layout .showcase-content,
.login-layout .graph-meta,
.login-layout .feature-tags { display: none !important; }
.login-layout .form-section {
  position: relative !important;
  z-index: 2 !important;
  display: block !important;
  width: min(460px, 100%) !important;
  flex: none !important;
  padding: 0 !important;
  margin: 0 auto !important;
}
.login-layout .form-panel { width: 100% !important; max-width: none !important; margin: 0 !important; }
.login-layout .form-card {
  padding: 42px 40px 34px !important;
  background: rgb(255 255 255 / 98%) !important;
  border: 1px solid #dbe8fb !important;
  border-radius: 18px !important;
  box-shadow: 0 24px 70px rgb(0 48 130 / 18%), 0 4px 12px rgb(0 48 130 / 8%) !important;
}
.login-layout .form-header { text-align: left !important; }
.login-layout .form-title { color: #0b2b5c !important; font-size: 30px !important; font-weight: 760 !important; }
.login-layout .form-content .t-input { min-height: 48px; border-color: #cbdaf0 !important; border-radius: 9px; }
.login-layout .form-content .t-input:hover,
.login-layout .form-content .t-input:focus-within { border-color: #1677ff !important; box-shadow: 0 0 0 3px rgb(22 119 255 / 12%) !important; }
.login-layout .submit-button { min-height: 48px; border-radius: 9px !important; background: #0052d9 !important; box-shadow: 0 8px 18px rgb(0 82 217 / 22%); }
.login-layout > .auth-footer { position: absolute !important; bottom: 24px; z-index: 2; color: #7890b3 !important; }
@media (max-width: 680px) {
  .login-layout .auth-topbar { padding: 0 20px !important; }
  .login-layout .auth-system__gateway, .login-layout .auth-system__status { display: none !important; }
  .login-layout .form-card { padding: 32px 24px 28px !important; }
}
:root[theme-mode="dark"] .login-layout.login-layout--gateway { background: #091a33 !important; }
:root[theme-mode="dark"] .login-layout.login-layout--gateway::before { background: linear-gradient(135deg, #001b4d, #003cab 62%, #0b5bd3); }
:root[theme-mode="dark"] .login-layout .form-card { background: #10233d !important; border-color: #234872 !important; }
:root[theme-mode="dark"] .login-layout .form-title { color: #f3f8ff !important; }
.login-layout.login-layout--gateway::before { z-index: 0 !important; }
.login-layout.login-layout--gateway > .form-section {
  grid-area: auto !important;
  position: relative !important;
  z-index: 2 !important;
  display: block !important;
  width: min(460px, 100%) !important;
  min-width: 0 !important;
  min-height: 0 !important;
  padding: 0 !important;
  margin: 0 auto !important;
  overflow: visible !important;
}
.login-layout.login-layout--gateway > .auth-topbar { z-index: 3 !important; }
.login-layout.login-layout--gateway > .auth-footer { z-index: 2 !important; }
#app .login-layout.login-layout--gateway > .form-section {
  grid-area: auto !important;
  display: block !important;
  width: 460px !important;
  max-width: calc(100% - 48px) !important;
  padding: 0 !important;
  margin: 0 auto !important;
  overflow: visible !important;
}
#app .login-layout.login-layout--gateway::before { z-index: 0 !important; }
#app .login-layout.login-layout--gateway > .showcase-section { display: none !important; }
#app .login-layout.login-layout--gateway > .auth-topbar { background: transparent !important; border-bottom: 0 !important; }
#app .login-layout.login-layout--gateway { background: linear-gradient(to bottom, #0052d9 0, #0052d9 84px, #f4f8ff 84px, #f4f8ff 100%) !important; }
#app .login-layout.login-layout--gateway .header-logo .logo-image { width: 34px !important; height: 34px !important; max-height: 34px !important; filter: brightness(0) invert(1) !important; }
</style>

<style lang="less">
/* Standalone FMind sign-in surface. This intentionally replaces the former
 * graph/showcase treatment without changing the authentication flow. */
.login-layout.login-layout--gateway {
  position: relative;
  display: flex !important;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  overflow: hidden;
  padding: 96px 24px 72px !important;
  background: #f4f8ff !important;
}

.login-layout.login-layout--gateway::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 46vh;
  background: linear-gradient(135deg, #003cab 0%, #0052d9 56%, #1677ff 100%);
  clip-path: polygon(0 0, 100% 0, 100% 70%, 78% 84%, 46% 74%, 18% 92%, 0 78%);
}

.login-layout.login-layout--gateway::after {
  content: '';
  position: absolute;
  width: 520px;
  height: 520px;
  right: -180px;
  top: -210px;
  border: 1px solid rgb(255 255 255 / 20%);
  border-radius: 50%;
  box-shadow: 0 0 0 42px rgb(255 255 255 / 5%), 0 0 0 84px rgb(255 255 255 / 4%);
}

.login-layout .auth-topbar {
  position: absolute !important;
  inset: 0 0 auto;
  height: 84px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 42px;
  z-index: 5;
}

.login-layout .header-logo {
  position: static !important;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: #fff;
  cursor: default;
}

.login-layout .header-logo .logo-image {
  width: 34px !important;
  height: 34px;
  object-fit: contain;
  filter: brightness(0) invert(1);
}

.login-layout .header-logo .logo-wordmark {
  color: #fff;
  font-size: 22px;
  font-weight: 750;
  letter-spacing: -.04em;
}

.login-layout .auth-system {
  display: flex;
  align-items: center;
  gap: 16px;
}

.login-layout .auth-system__gateway {
  color: rgb(255 255 255 / 78%);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: .12em;
}

.login-layout .auth-system__status {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: rgb(255 255 255 / 88%);
  font-size: 11px;
  font-weight: 650;
  letter-spacing: .08em;
}

.login-layout .auth-system__status i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #7de3b2;
  box-shadow: 0 0 0 4px rgb(125 227 178 / 18%);
}

.login-layout .language-switch .header-link {
  min-height: 38px;
  padding: 8px 12px;
  color: #fff;
  background: rgb(255 255 255 / 12%);
  border: 1px solid rgb(255 255 255 / 24%);
  border-radius: 8px;
}

.login-layout .language-switch .header-link:hover {
  background: rgb(255 255 255 / 20%);
  border-color: rgb(255 255 255 / 40%);
}

.login-layout .showcase-section,
.login-layout .showcase-content,
.login-layout .graph-meta,
.login-layout .feature-tags {
  display: none !important;
}

.login-layout .form-section {
  position: relative;
  z-index: 2;
  display: block !important;
  flex: none;
  width: min(100%, 460px);
  padding: 0 !important;
}

.login-layout .form-panel {
  width: 100%;
  max-width: none;
  margin: 0;
}

.login-layout .form-card {
  padding: 42px 40px 34px !important;
  background: rgb(255 255 255 / 98%) !important;
  border: 1px solid #dbe8fb !important;
  border-radius: 18px !important;
  box-shadow: 0 24px 70px rgb(0 48 130 / 18%), 0 4px 12px rgb(0 48 130 / 8%) !important;
}

.login-layout .form-header {
  margin-bottom: 28px;
  text-align: left !important;
}

.login-layout .form-title {
  color: #0b2b5c !important;
  font-size: 30px !important;
  font-weight: 760 !important;
  letter-spacing: -.045em;
}

.login-layout .form-welcome,
.login-layout .form-subtitle {
  color: #60718d !important;
  font-size: 14px !important;
}

.login-layout .form-hint {
  color: #1254b5 !important;
  background: #edf5ff !important;
  border: 1px solid #d4e7ff;
}

.login-layout .form-content .t-form-item__label {
  color: #253b5c;
  font-size: 13px;
  font-weight: 650;
}

.login-layout .form-content .t-input {
  min-height: 48px;
  border-color: #cbdaf0 !important;
  border-radius: 9px;
  background: #fff;
}

.login-layout .form-content .t-input:hover,
.login-layout .form-content .t-input:focus-within {
  border-color: #1677ff !important;
  box-shadow: 0 0 0 3px rgb(22 119 255 / 12%) !important;
}

.login-layout .submit-button {
  min-height: 48px;
  margin-top: 8px;
  border-radius: 9px !important;
  background: #0052d9 !important;
  box-shadow: 0 8px 18px rgb(0 82 217 / 22%);
  font-size: 15px;
  font-weight: 700;
}

.login-layout .submit-button:hover {
  background: #1677ff !important;
  transform: translateY(-1px);
}

.login-layout .register-cta__button,
.login-layout .oidc-button {
  min-height: 46px;
  border-radius: 9px !important;
}

.login-layout .login-features {
  display: grid;
  gap: 9px;
  margin-top: 28px;
  padding-top: 20px !important;
  border-top: 1px solid #e6eef9 !important;
}

.login-layout .feature-item {
  color: #60718d;
  font-size: 12px;
}

.login-layout .feature-icon {
  display: inline-grid;
  width: 20px;
  height: 20px;
  place-items: center;
  color: #0052d9 !important;
  background: #eaf3ff !important;
  border-radius: 50%;
}

.login-layout > .auth-footer {
  position: absolute;
  bottom: 24px;
  z-index: 2;
  color: #7890b3;
  font-size: 12px;
  letter-spacing: .04em;
}

@media (max-width: 680px) {
  .login-layout .auth-topbar { padding: 0 20px; }
  .login-layout .auth-system__gateway,
  .login-layout .auth-system__status { display: none; }
  .login-layout .form-card { padding: 32px 24px 28px !important; }
}

:root[theme-mode="dark"] .login-layout.login-layout--gateway {
  background: #091a33 !important;
}

:root[theme-mode="dark"] .login-layout.login-layout--gateway::before {
  background: linear-gradient(135deg, #001b4d 0%, #003cab 62%, #0b5bd3 100%);
}

:root[theme-mode="dark"] .login-layout .form-card {
  background: #10233d !important;
  border-color: #234872 !important;
  box-shadow: 0 24px 70px rgb(0 0 0 / 40%) !important;
}

:root[theme-mode="dark"] .login-layout .form-title { color: #f3f8ff !important; }
:root[theme-mode="dark"] .login-layout .form-welcome,
:root[theme-mode="dark"] .login-layout .form-subtitle,
:root[theme-mode="dark"] .login-layout .feature-item { color: #a9c1df !important; }
:root[theme-mode="dark"] .login-layout .form-content .t-form-item__label { color: #d9e8fb; }
:root[theme-mode="dark"] .login-layout .form-content .t-input { background: #0b1b31; border-color: #31547d !important; }
:root[theme-mode="dark"] .login-layout .login-features { border-top-color: #29486d !important; }
</style>

<style lang="less">
html[theme-mode="dark"] {
  .login-layout {
    background: linear-gradient(225deg, #080b1a 0%, #10132d 15%, #171b3f 25%, #1e2254 38%, #282c6b 50%, #34398b 65%, #3b4aa8 78%, #4f5fc5 90%, #6366d9 100%);
  }

  .knowledge-node {
    background: rgba(255, 255, 255, 0.1);
    border-color: rgba(255, 255, 255, 0.2);
    box-shadow: 0 0 8px rgba(255, 255, 255, 0.15);
  }

  .connection-line {
    stroke: rgba(255, 255, 255, 0.25);
  }

  .header-logo .logo-image {
    filter: invert(1) hue-rotate(180deg) brightness(1.1);
  }

  .header-link {
    background: rgba(255, 255, 255, 0.12);
    border-color: rgba(255, 255, 255, 0.15);

    &:hover {
      background: rgba(255, 255, 255, 0.2);
    }
  }

  .language-switch button {
    background: rgba(255, 255, 255, 0.12);
    border-color: rgba(255, 255, 255, 0.15);

    &:hover {
      background: rgba(255, 255, 255, 0.2);
    }
  }

  .language-dropdown {
    background: rgba(36, 36, 36, 0.97) !important;
    border-color: var(--td-component-stroke) !important;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4) !important;
  }

  .tag {
    background: rgba(255, 255, 255, 0.12);
  }

  .form-card {
    background: rgba(36, 36, 36, 0.97) !important;
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.4) !important;
  }

  .register-cta__divider span {
    background: rgba(36, 36, 36, 0.97);
  }

  .form-content .t-input {
    background: var(--td-bg-color-page) !important;
    border-color: rgba(255, 255, 255, 0.1) !important;

    &:hover {
      border-color: var(--td-brand-color) !important;
    }

    &:focus-within {
      border-color: var(--td-brand-color) !important;
    }
  }

  .screenshot-swiper .swiper-pagination-bullet-active {
    background: rgba(255, 255, 255, 0.9) !important;
  }

  .login-features .feature-icon {
    background: rgba(6, 176, 77, 0.15);
  }
}
</style>

<style lang="less">
.login-layout.login-layout--gateway {
  position: relative;
  display: grid !important;
  grid-template-columns: minmax(0, 1.08fr) minmax(410px, 0.72fr);
  grid-template-rows: 82px minmax(0, 1fr);
  place-items: stretch !important;
  width: 100%;
  height: 100dvh;
  min-height: 620px !important;
  padding: 0 !important;
  overflow: hidden;
  color: #151824;
  background: #fbfcff !important;

  &::before {
    display: none;
  }

  > .fmind-auth-graph {
    position: absolute;
    inset: 0;
    z-index: 0;
  }

  > .auth-topbar {
    position: relative;
    z-index: 10;
    grid-column: 1 / -1;
    grid-row: 1;
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-width: 0;
    padding: 0 clamp(24px, 4vw, 68px);
    border-bottom: 1px solid rgba(219, 226, 255, 0.82);
    background: rgba(255, 255, 255, 0.76);
    box-shadow: 0 1px 0 rgba(255, 255, 255, 0.68) inset;
    backdrop-filter: blur(16px) saturate(132%);
  }

  .header-logo {
    position: static !important;
    display: flex;
    align-items: center;
    height: 54px;
    cursor: default;

    .logo-image {
      display: block;
      width: clamp(158px, 12vw, 194px) !important;
      height: auto;
      max-height: 54px;
      object-fit: contain;
      filter: none !important;
    }
  }

  .auth-system {
    display: flex;
    align-items: center;
    gap: 18px;
    color: #70778a;
    font: 10px ui-monospace, SFMono-Regular, Consolas, monospace;
    letter-spacing: 0.07em;
    white-space: nowrap;
  }

  .auth-system__status {
    display: flex;
    align-items: center;
    gap: 8px;
    color: #4f5872;

    i {
      width: 7px;
      height: 7px;
      border-radius: 50%;
      background: #3447ff;
      box-shadow: 0 0 0 5px rgba(52, 71, 255, 0.1);
      /* Keep the authentication landing page visually stable. The pulse
       * caused the whole header to appear to flicker on some Chromium/WSL2
       * compositors, especially while the page was still loading auth config. */
      animation: none;
    }
  }

  .language-switch {
    position: relative;
  }

  .header-link {
    display: flex;
    align-items: center;
    gap: 7px;
    min-height: 34px;
    padding: 7px 11px;
    color: #50586c;
    font-size: 12px;
    font-weight: 500;
    border: 1px solid #dfe4f5;
    border-radius: 7px;
    background: rgba(255, 255, 255, 0.78);
    cursor: pointer;
    transition: border-color 0.2s ease, background 0.2s ease, color 0.2s ease;

    &:hover {
      color: #3447ff;
      border-color: rgba(52, 71, 255, 0.35);
      background: #fff;
    }
  }

  .language-dropdown {
    position: absolute;
    top: calc(100% + 10px);
    right: 0;
    z-index: 1000;
    min-width: 168px;
    padding: 5px;
    overflow: hidden;
    border: 1px solid #dfe4f5;
    border-radius: 9px;
    background: rgba(255, 255, 255, 0.97);
    box-shadow: 0 18px 48px rgba(24, 39, 102, 0.14);
    backdrop-filter: blur(16px);
  }

  .language-option {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 9px 11px;
    color: #42495d;
    font-size: 13px;
    border-radius: 6px;
    cursor: pointer;

    .lang-label {
      flex: 1;
    }

    .check-icon {
      color: #3447ff;
    }

    &:hover,
    &.active {
      color: #3447ff;
      background: #f0f2ff;
    }
  }

  > .showcase-section {
    position: relative;
    z-index: 3;
    grid-column: 1;
    grid-row: 2;
    display: flex !important;
    align-items: flex-end;
    min-width: 0;
    min-height: 0;
    padding: 42px clamp(28px, 4.5vw, 74px) 48px !important;
    pointer-events: none;
  }

  .showcase-content {
    position: relative;
    z-index: 3;
    display: block;
    width: min(520px, 100%);
    max-width: 520px;
    margin: 0 !important;
    padding: 20px 22px 18px;
    color: #151824;
    border: 1px solid rgba(210, 219, 255, 0.72);
    border-radius: 12px;
    background: rgba(255, 255, 255, 0.64);
    box-shadow: 0 18px 52px rgba(19, 47, 154, 0.09);
    backdrop-filter: blur(16px) saturate(122%);
  }

  .showcase-overline {
    display: flex;
    align-items: center;
    gap: 9px;
    margin-bottom: 11px;
    color: #3447ff;
    font: 10px ui-monospace, SFMono-Regular, Consolas, monospace;
    letter-spacing: 0.1em;

    span {
      width: 24px;
      height: 1px;
      background: #3447ff;
    }
  }

  .showcase-title {
    margin: 0;
    color: #151824;
    font-size: clamp(26px, 2.45vw, 38px);
    font-weight: 650;
    line-height: 1.24;
    letter-spacing: -0.05em;
  }

  .showcase-description {
    max-width: 470px;
    margin: 12px 0 0;
    color: #697184;
    font-size: 13px;
    line-height: 1.75;
  }

  .feature-tags {
    display: flex;
    gap: 7px;
    margin: 16px 0 0;
    flex-wrap: wrap;
  }

  .tag {
    display: inline-flex;
    align-items: center;
    min-height: 25px;
    padding: 4px 9px;
    color: #4f5d88;
    font-size: 11px;
    font-weight: 500;
    border: 1px solid rgba(52, 71, 255, 0.13);
    border-radius: 6px;
    background: rgba(240, 242, 255, 0.72);
  }

  .graph-meta {
    display: flex;
    align-items: center;
    gap: 14px;
    margin-top: 17px;
    color: #8a91a4;
    font: 9px ui-monospace, SFMono-Regular, Consolas, monospace;
    letter-spacing: 0.055em;
    white-space: nowrap;

    strong {
      color: #555e77;
      font-weight: 500;
    }
  }

  .graph-meta__pulse {
    width: 6px;
    height: 6px;
    flex: 0 0 auto;
    border-radius: 50%;
    background: #3447ff;
    box-shadow: 0 0 0 4px rgba(52, 71, 255, 0.09);
  }

  > .form-section {
    position: relative;
    z-index: 4;
    grid-column: 2;
    grid-row: 2;
    display: flex !important;
    align-items: center;
    justify-content: center;
    width: auto !important;
    min-width: 0;
    min-height: 0 !important;
    padding: 34px clamp(28px, 4.8vw, 72px) 46px !important;
    overflow: auto;
    scrollbar-width: thin;
    scrollbar-color: rgba(52, 71, 255, 0.2) transparent;
  }

  .form-panel {
    position: relative;
    z-index: 2;
    width: min(430px, 100%) !important;
    max-width: 430px !important;
    margin: auto !important;
    padding: 0 !important;
    background: transparent !important;
  }

  .form-card {
    width: 100% !important;
    padding: 30px !important;
    color: #181b26;
    border: 1px solid rgba(214, 222, 252, 0.9) !important;
    border-radius: 13px !important;
    background: rgba(255, 255, 255, 0.91) !important;
    box-shadow:
      0 28px 68px rgba(18, 38, 121, 0.14),
      0 0 0 1px rgba(255, 255, 255, 0.68) inset !important;
    backdrop-filter: blur(22px) saturate(126%);
  }

  .form-header {
    margin-bottom: 24px;
    text-align: left !important;
  }

  .form-title {
    margin: 0 0 7px;
    color: #171a25 !important;
    font-size: 28px !important;
    font-weight: 650;
    line-height: 1.1 !important;
    letter-spacing: -0.045em !important;
  }

  .form-welcome,
  .form-subtitle {
    margin: 0;
    color: #70778a !important;
    font-size: 12px !important;
    line-height: 1.55 !important;
  }

  .form-hint {
    margin: 12px 0 0;
    padding: 9px 11px;
    color: #4e5b84 !important;
    font-size: 11px !important;
    line-height: 1.55 !important;
    border: 1px solid #dce3ff;
    border-radius: 7px;
    background: #f4f6ff !important;
  }

  .form-content {
    .t-form-item__label {
      margin-bottom: 6px;
      color: #454b5e;
      font-size: 12px;
      font-weight: 600;
    }

    .t-form-item {
      margin-bottom: 15px;
    }

    .t-input {
      min-height: 42px;
      border-color: #dfe4f3;
      border-radius: 7px;
      background: rgba(255, 255, 255, 0.88);

      &:hover,
      &:focus-within {
        border-color: #3447ff;
      }

      &:focus-within {
        box-shadow: 0 0 0 3px rgba(52, 71, 255, 0.08);
      }
    }
  }

  .submit-button,
  .oidc-button,
  .register-cta__button {
    height: 42px;
    border-radius: 7px;
    font-size: 13px;
  }

  .submit-button {
    margin: 17px 0 15px;
    box-shadow: 0 9px 22px rgba(52, 71, 255, 0.18);
  }

  .register-cta__divider,
  .oidc-divider {
    margin: 3px 0 12px;
    color: #969cac;
    font-size: 11px;

    span {
      background: rgba(255, 255, 255, 0.91);
    }
  }

  .login-features {
    margin-top: 18px;
    padding-top: 14px !important;
    border-top: 1px solid #edf0f6 !important;

    .feature-item {
      margin-bottom: 9px;
      color: #7a8193;
      font-size: 11px;
    }

    .feature-icon {
      width: 18px;
      height: 18px;
      margin-right: 8px;
      color: #3447ff !important;
      font-size: 10px;
      background: #eef0ff !important;
    }
  }

  > .auth-footer {
    position: absolute;
    z-index: 3;
    right: clamp(24px, 3vw, 52px);
    bottom: 18px;
    color: #9aa0ae;
    font: 9px ui-monospace, SFMono-Regular, Consolas, monospace;
    letter-spacing: 0.04em;
    pointer-events: none;
  }
}

@keyframes authStatusPulse {
  50% {
    box-shadow: 0 0 0 10px rgba(52, 71, 255, 0);
  }
}

@media (max-width: 1080px) {
  .login-layout.login-layout--gateway {
    grid-template-columns: minmax(0, 1fr) minmax(390px, 0.82fr);

    .graph-meta__hint,
    .graph-meta > span:nth-last-child(2) {
      display: none;
    }

    .auth-system__gateway {
      display: none;
    }
  }
}

@media (max-width: 820px) {
  .login-layout.login-layout--gateway {
    grid-template-columns: minmax(0, 1fr);

    > .showcase-section {
      display: none !important;
    }

    > .form-section {
      grid-column: 1;
      padding-inline: 24px !important;
    }

    > .fmind-auth-graph {
      opacity: 0.92;
    }
  }
}

@media (max-width: 540px) {
  .login-layout.login-layout--gateway {
    grid-template-rows: 68px minmax(0, 1fr);
    min-height: 100dvh !important;

    > .auth-topbar {
      padding: 0 18px;
    }

    .header-logo .logo-image {
      width: 142px !important;
    }

    .auth-system__status {
      display: none;
    }

    .header-link {
      padding: 7px 9px;

      .link-text {
        display: none;
      }
    }

    > .form-section {
      padding: 22px 16px 38px !important;
    }

    .form-card {
      padding: 24px 21px !important;
      border-radius: 11px !important;
    }

    > .auth-footer {
      display: none;
    }
  }
}

@media (prefers-reduced-motion: reduce) {
  .login-layout.login-layout--gateway .auth-system__status i {
    animation: none;
  }
}

html[theme-mode='dark'] {
  .login-layout.login-layout--gateway {
    color: #eef1ff;
    background: #0c1020 !important;

    > .auth-topbar {
      border-bottom-color: rgba(129, 140, 248, 0.18);
      background: rgba(12, 16, 32, 0.74);
      box-shadow: 0 1px 0 rgba(129, 140, 248, 0.05) inset;
    }

    .header-logo .logo-image {
      filter: invert(1) brightness(1.08) !important;
    }

    .auth-system,
    .auth-system__status {
      color: #a9b1ca;
    }

    .header-link {
      color: #c4cae0;
      border-color: rgba(129, 140, 248, 0.22);
      background: rgba(19, 25, 49, 0.78);

      &:hover {
        color: #c7d2fe;
        border-color: rgba(129, 140, 248, 0.48);
        background: rgba(29, 36, 68, 0.92);
      }
    }

    .language-dropdown {
      border-color: rgba(129, 140, 248, 0.22);
      background: rgba(18, 23, 45, 0.97) !important;
      box-shadow: 0 18px 48px rgba(0, 0, 0, 0.36);
    }

    .language-option {
      color: #c4cae0;

      &:hover,
      &.active {
        color: #c7d2fe;
        background: rgba(99, 102, 241, 0.15);
      }
    }

    .showcase-content {
      color: #eef1ff;
      border-color: rgba(129, 140, 248, 0.2);
      background: rgba(14, 19, 39, 0.64);
      box-shadow: 0 18px 52px rgba(0, 0, 0, 0.24);
    }

    .showcase-title {
      color: #f3f5ff;
    }

    .showcase-description {
      color: #aeb7d0;
    }

    .tag {
      color: #c6cced;
      border-color: rgba(129, 140, 248, 0.2);
      background: rgba(99, 102, 241, 0.12);
    }

    .graph-meta,
    .graph-meta strong {
      color: #929bb6;
    }

    .form-card {
      color: #eef1ff;
      border-color: rgba(129, 140, 248, 0.2) !important;
      background: rgba(16, 21, 42, 0.91) !important;
      box-shadow:
        0 28px 68px rgba(0, 0, 0, 0.38),
        0 0 0 1px rgba(129, 140, 248, 0.04) inset !important;
    }

    .form-title {
      color: #f2f4ff !important;
    }

    .form-welcome,
    .form-subtitle {
      color: #aeb7d0 !important;
    }

    .form-hint {
      color: #bdc7ed !important;
      border-color: rgba(129, 140, 248, 0.2);
      background: rgba(99, 102, 241, 0.12) !important;
    }

    .form-content {
      .t-form-item__label {
        color: #c8cee2;
      }

      .t-input {
        border-color: rgba(129, 140, 248, 0.18) !important;
        background: rgba(8, 12, 27, 0.72) !important;
      }
    }

    .register-cta__divider span,
    .oidc-divider span {
      background: rgba(16, 21, 42, 0.91);
    }

    .login-features {
      border-top-color: rgba(129, 140, 248, 0.14) !important;

      .feature-item {
        color: #aeb7d0;
      }

      .feature-icon {
        color: #a5b4fc !important;
        background: rgba(99, 102, 241, 0.16) !important;
      }
    }

    > .auth-footer {
      color: #747d99;
    }
  }
}
</style>

<style lang="less">
/* Ensure the standalone composition wins over the legacy login rules. */
.login-layout.login-layout--gateway { display: flex !important; align-items: center !important; justify-content: center !important; min-height: 100vh !important; padding: 96px 24px 72px !important; background: #f4f8ff !important; }
.login-layout.login-layout--gateway::before { content: ''; position: absolute; inset: 0 0 auto; height: 44vh; background: linear-gradient(135deg, #003cab, #0052d9 58%, #1677ff); clip-path: polygon(0 0, 100% 0, 100% 72%, 78% 88%, 48% 76%, 19% 91%, 0 78%); }
.login-layout .auth-topbar { position: absolute !important; inset: 0 0 auto !important; height: 84px !important; padding: 0 42px !important; z-index: 5 !important; }
.login-layout .header-logo { position: static !important; display: inline-flex !important; align-items: center; gap: 10px; color: #fff !important; }
.login-layout .header-logo .logo-image { width: 34px !important; height: 34px !important; object-fit: contain; filter: brightness(0) invert(1) !important; }
.login-layout .header-logo .logo-wordmark { color: #fff !important; font-size: 22px; font-weight: 750; }
.login-layout .showcase-section, .login-layout .showcase-content, .login-layout .graph-meta, .login-layout .feature-tags { display: none !important; }
.login-layout .form-section { position: relative !important; z-index: 2 !important; display: block !important; width: min(460px, 100%) !important; flex: none !important; padding: 0 !important; margin: 0 auto !important; }
.login-layout .form-panel { width: 100% !important; max-width: none !important; margin: 0 !important; }
.login-layout .form-card { padding: 42px 40px 34px !important; background: rgb(255 255 255 / 98%) !important; border: 1px solid #dbe8fb !important; border-radius: 18px !important; box-shadow: 0 24px 70px rgb(0 48 130 / 18%), 0 4px 12px rgb(0 48 130 / 8%) !important; }
.login-layout .form-header { text-align: left !important; }
.login-layout .form-title { color: #0b2b5c !important; font-size: 30px !important; font-weight: 760 !important; }
.login-layout .form-content .t-input { min-height: 48px; border-color: #cbdaf0 !important; border-radius: 9px; }
.login-layout .form-content .t-input:hover, .login-layout .form-content .t-input:focus-within { border-color: #1677ff !important; box-shadow: 0 0 0 3px rgb(22 119 255 / 12%) !important; }
.login-layout .submit-button { min-height: 48px; border-radius: 9px !important; background: #0052d9 !important; box-shadow: 0 8px 18px rgb(0 82 217 / 22%); }
.login-layout > .auth-footer { position: absolute !important; bottom: 24px; z-index: 2; color: #7890b3 !important; }
@media (max-width: 680px) { .login-layout .auth-topbar { padding: 0 20px !important; } .login-layout .auth-system__gateway, .login-layout .auth-system__status { display: none !important; } .login-layout .form-card { padding: 32px 24px 28px !important; } }
:root[theme-mode="dark"] .login-layout.login-layout--gateway { background: #091a33 !important; }
:root[theme-mode="dark"] .login-layout.login-layout--gateway::before { background: linear-gradient(135deg, #001b4d, #003cab 62%, #0b5bd3); }
:root[theme-mode="dark"] .login-layout .form-card { background: #10233d !important; border-color: #234872 !important; }
:root[theme-mode="dark"] .login-layout .form-title { color: #f3f8ff !important; }
</style>
