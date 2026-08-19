<template>
  <main class="memory-admin" aria-labelledby="memory-admin-title">
    <header class="memory-admin__header">
      <div>
        <p class="memory-admin__eyebrow">FTMind · {{ $t('externalMemory.eyebrow') }}</p>
        <h1 id="memory-admin-title">{{ $t('externalMemory.title') }}</h1>
        <p>{{ $t('externalMemory.subtitle') }}</p>
      </div>
      <t-button theme="primary" :loading="loading" @click="reload">
        <template #icon><t-icon name="refresh" /></template>{{ $t('externalMemory.refresh') }}
      </t-button>
    </header>

    <t-alert theme="info" :message="$t('externalMemory.scopeNote')" closeable class="memory-admin__notice" />

    <section v-if="isOverview" class="memory-overview">
      <div class="overview-grid">
        <article class="overview-card"><span>{{ $t('externalMemory.overview.activeBindings') }}</span><strong>{{ activeBindingCount }}</strong><small>{{ $t('externalMemory.overview.activeBindingsHint') }}</small></article>
        <article class="overview-card"><span>{{ $t('externalMemory.overview.revokedBindings') }}</span><strong>{{ revokedBindingCount }}</strong><small>{{ $t('externalMemory.overview.revokedBindingsHint') }}</small></article>
        <article class="overview-card overview-card--warning"><span>{{ $t('externalMemory.overview.pendingReviews') }}</span><strong>{{ pendingReviewCount }}</strong><small>{{ $t('externalMemory.overview.pendingReviewsHint') }}</small></article>
        <article class="overview-card overview-card--success"><span>{{ $t('externalMemory.overview.publishedReviews') }}</span><strong>{{ publishedReviewCount }}</strong><small>{{ $t('externalMemory.overview.publishedReviewsHint') }}</small></article>
      </div>
      <div class="overview-actions">
        <div><h2>{{ $t('externalMemory.overview.title') }}</h2><p>{{ $t('externalMemory.overview.desc') }}</p></div>
        <div class="overview-action-buttons"><t-button v-if="canCreateBinding" theme="primary" @click="goToCreate"><template #icon><t-icon name="user-add" /></template>{{ $t('externalMemory.bindings.create') }}</t-button><t-button v-if="canReadBindings" variant="outline" @click="goTo('bindings')"><template #icon><t-icon name="control-platform" /></template>{{ $t('externalMemory.overview.openBindings') }}</t-button><t-button v-if="canReadMemory" variant="outline" @click="goTo('reviews')"><template #icon><t-icon name="task" /></template>{{ $t('externalMemory.overview.openReviews') }}</t-button></div>
      </div>
      <t-alert theme="info" :message="$t('externalMemory.overview.wikiHint')" />
    </section>

    <t-tabs v-if="!isOverview" v-model="activeTab" class="memory-admin__tabs">
      <t-tab-panel v-if="activeTab === 'bindings'" value="bindings" :label="$t('externalMemory.tabs.bindings')">
        <section class="memory-admin__section" aria-labelledby="bindings-title">
          <div class="section-heading">
            <div><h2 id="bindings-title">{{ $t('externalMemory.bindings.title') }}</h2><p>{{ $t('externalMemory.bindings.desc') }}</p></div>
            <t-button v-if="canCreateBinding" theme="primary" @click="openCreate"><template #icon><t-icon name="add" /></template>{{ $t('externalMemory.bindings.create') }}</t-button>
          </div>
          <t-table :data="bindings" :columns="bindingColumns" row-key="id" :loading="loading" :pagination="false" :empty="$t('externalMemory.bindings.empty')">
            <template #status="{ row }"><t-tag :theme="row.status === 'active' ? 'success' : row.status === 'pending_setup' ? 'warning' : 'default'">{{ statusLabel(row.status) }}</t-tag></template>
            <template #policy="{ row }"><div class="policy-tags"><t-tag v-if="row.capture_enabled" variant="light">{{ $t('externalMemory.policy.capture') }}</t-tag><t-tag v-if="row.recall_enabled" variant="light">{{ $t('externalMemory.policy.recall') }}</t-tag><t-tag v-if="row.l3_wiki_enabled" theme="primary" variant="light">L3 Wiki</t-tag></div></template>
            <template #actions="{ row }"><div class="table-actions"><t-button v-if="canRotateBinding" size="small" variant="text" :disabled="!['active', 'pending_setup'].includes(row.status)" @click="rotate(row)">{{ $t('externalMemory.bindings.rotate') }}</t-button><t-button v-if="canRevokeBinding" size="small" variant="text" theme="danger" :disabled="!['active', 'pending_setup'].includes(row.status)" @click="confirmRevoke(row)">{{ $t('externalMemory.bindings.revoke') }}</t-button></div></template>
          </t-table>
        </section>
      </t-tab-panel>

      <t-tab-panel v-if="activeTab === 'reviews'" value="reviews" :label="$t('externalMemory.tabs.reviews')">
        <section class="memory-admin__section" aria-labelledby="reviews-title">
          <div class="section-heading section-heading--review">
            <div><h2 id="reviews-title">{{ $t('externalMemory.reviews.title') }}</h2><p>{{ $t('externalMemory.reviews.desc') }}</p></div>
            <t-select v-model="reviewStatus" :options="reviewStatusOptions" clearable :placeholder="$t('externalMemory.reviews.allStatuses')" @change="loadReviews()" />
          </div>
          <t-table :data="reviews" :columns="reviewColumns" row-key="id" :loading="reviewLoading" :pagination="false" :empty="$t('externalMemory.reviews.empty')">
            <template #status="{ row }"><t-tag :theme="reviewTheme(row.status)">{{ statusLabel(row.status) }}</t-tag></template>
            <template #actions="{ row }"><t-button size="small" variant="text" @click="openReview(row)">{{ $t('externalMemory.reviews.open') }}</t-button></template>
          </t-table>
        </section>
      </t-tab-panel>
    </t-tabs>

    <t-dialog v-model:visible="createVisible" class="binding-dialog" :header="$t('externalMemory.bindings.createTitle')" :confirm-btn="$t('externalMemory.bindings.create')" :cancel-btn="$t('common.cancel')" :confirm-loading="savingBinding" width="760px" @confirm="submitBindingForm">
      <t-form ref="bindingFormRef" :data="bindingForm" :rules="bindingRules" label-align="top" @submit.prevent>
        <div class="form-grid">
          <t-form-item name="department_id" :label="$t('externalMemory.fields.department')"><t-select v-model="selectedDepartmentId" :options="departmentOptions" :loading="organizationLoading" :placeholder="$t('externalMemory.fields.selectDepartment')" /></t-form-item>
          <t-form-item name="team_id" :label="$t('externalMemory.fields.teamId')" :help="teams.length ? '' : $t('externalMemory.fields.noTeams')"><t-select v-model="bindingForm.team_id" :options="teamOptions" :disabled="!selectedDepartmentId || teams.length === 0" :placeholder="$t('externalMemory.fields.selectTeam')" /></t-form-item>
          <t-form-item name="agent_id" :label="$t('externalMemory.fields.agentId')"><t-select v-model="bindingForm.agent_id" :options="agentOptions" :disabled="!bindingForm.team_id || teamAgents.length === 0" :placeholder="$t('externalMemory.fields.selectAgent')" /></t-form-item>
          <t-form-item name="external_agent" :label="$t('externalMemory.fields.externalAgent')"><t-input v-model="bindingForm.external_agent" autocomplete="off" /></t-form-item>
          <t-form-item name="connector_type" :label="$t('externalMemory.fields.connector')"><t-select v-model="bindingForm.connector_type" :options="connectorOptions" /></t-form-item>
        </div>
        <t-form-item name="user_api_key_id" :label="$t('externalMemory.fields.userApiKey', '用户 API Key')" :help="$t('externalMemory.fields.userApiKeyHelp', '选择绑定用户已有的 FTMind 用户 API Key；提示词只使用环境变量占位符。')"><t-select v-model="bindingForm.user_api_key_id" :options="userApiKeyOptions" :loading="apiKeysLoading" :placeholder="$t('externalMemory.fields.selectUserApiKey', '请选择用户 API Key')" /></t-form-item>
        <t-form-item :label="$t('externalMemory.fields.userId')"><t-select v-model="bindingForm.user_id" :options="userOptions" :disabled="!bindingForm.team_id || teamMembers.length === 0" :placeholder="$t('externalMemory.fields.selectUser')" /></t-form-item>
        <t-form-item class="binding-capabilities" :label="$t('externalMemory.fields.capabilities')"><t-checkbox-group v-model="bindingForm.capability_scopes" :options="capabilityOptions" /></t-form-item>
        <t-form-item :label="$t('externalMemory.fields.assetScopes')" :help="$t('externalMemory.fields.assetScopesHelp')"><t-textarea v-model="assetScopesText" :autosize="{ minRows: 2, maxRows: 4 }" /></t-form-item>
        <div class="switch-grid"><t-checkbox v-model="bindingForm.capture_enabled">{{ $t('externalMemory.policy.capture') }}</t-checkbox><t-checkbox v-model="bindingForm.recall_enabled">{{ $t('externalMemory.policy.recall') }}</t-checkbox><t-checkbox v-model="bindingForm.l3_wiki_enabled">{{ $t('externalMemory.policy.l3') }}</t-checkbox></div>
      </t-form>
    </t-dialog>

    <t-dialog v-model:visible="secretVisible" :header="setupPrompt ? $t('externalMemory.setupWizard.title') : $t('externalMemory.secret.title')" :confirm-btn="$t('externalMemory.secret.confirm')" :cancel-btn="null" width="900px" @confirm="closeSecret">
      <t-alert :theme="setupPrompt ? 'info' : 'warning'" :message="setupPrompt ? $t('externalMemory.setupWizard.warning') : $t('externalMemory.secret.warning')" />
      <template v-if="setupPrompt">
        <div class="setup-wizard__status"><t-tag theme="warning">{{ $t('externalMemory.status.pending_setup') }}</t-tag><span>{{ $t('externalMemory.setupWizard.expires') }} {{ setupExpiresAt ? new Date(setupExpiresAt).toLocaleString() : '-' }}</span></div>
        <p class="setup-wizard__hint">{{ $t('externalMemory.setupWizard.instructions') }}</p>
        <div class="setup-prompt-box"><pre>{{ setupPrompt }}</pre><t-button theme="primary" @click="copyPrompt"><template #icon><t-icon name="file-copy" /></template>{{ $t('externalMemory.setupWizard.copyPrompt') }}</t-button></div>
      </template>
      <div v-else class="secret-box"><code>{{ connectorSecret }}</code><t-button variant="text" :aria-label="$t('externalMemory.secret.copy')" @click="copySecret"><template #icon><t-icon name="file-copy" /></template>{{ $t('externalMemory.secret.copy') }}</t-button></div>
    </t-dialog>

    <t-dialog v-model:visible="revokeVisible" :header="$t('externalMemory.bindings.revokeTitle')" theme="warning" :confirm-btn="$t('externalMemory.bindings.revoke')" :confirm-loading="revoking" @confirm="revoke">
      {{ $t('externalMemory.bindings.revokeHint', { name: selectedBinding?.external_agent || '' }) }}
    </t-dialog>
    <t-dialog v-model:visible="memoryRevokeVisible" :header="$t('externalMemory.bindings.revokeTitle')" theme="warning" :confirm-btn="$t('externalMemory.bindings.revoke')" :confirm-loading="deciding" @confirm="revokeMemoryProjection">
      <t-form label-align="top"><t-form-item :label="$t('externalMemory.reviews.comment')"><t-textarea v-model="decisionComment" :autosize="{ minRows: 3, maxRows: 6 }" /></t-form-item></t-form>
    </t-dialog>

    <t-drawer v-model:visible="reviewVisible" size="720px" :header="$t('externalMemory.reviews.detailTitle')" :footer="false" @close="selectedReview = null">
      <template v-if="selectedReview"><div class="review-detail"><div class="review-detail__top"><div><h2>{{ selectedReview.title }}</h2><p>{{ selectedReview.team_id }} · {{ selectedReview.agent_id }}</p></div><t-tag :theme="reviewTheme(selectedReview.status)">{{ statusLabel(selectedReview.status) }}</t-tag></div><dl class="review-meta"><div><dt>Memory ID</dt><dd>{{ selectedReview.memory_id }}</dd></div><div><dt>{{ $t('externalMemory.reviews.reviewComment') }}</dt><dd>{{ selectedReview.review_comment || '—' }}</dd></div></dl><section><h3>{{ $t('externalMemory.reviews.markdown') }}</h3><pre class="markdown-preview">{{ selectedReview.markdown }}</pre></section><section><h3>{{ $t('externalMemory.reviews.evidence') }}</h3><ul v-if="selectedReview.evidence?.length" class="evidence-list"><li v-for="item in selectedReview.evidence" :key="item">{{ item }}</li></ul><p v-else class="muted">{{ $t('externalMemory.reviews.noEvidence') }}</p></section><div class="review-actions"><t-button v-if="canReview" theme="primary" @click="openDecision('approve')">{{ $t('externalMemory.actions.approve') }}</t-button><t-button v-if="canReview" variant="outline" @click="openDecision('request_changes')">{{ $t('externalMemory.actions.requestChanges') }}</t-button><t-button v-if="canReview" theme="danger" variant="outline" @click="openDecision('reject')">{{ $t('externalMemory.actions.reject') }}</t-button><t-button v-if="canPublish" theme="primary" @click="publish">{{ $t('externalMemory.actions.publish') }}</t-button><t-button v-if="canRevokeMemory" theme="danger" variant="outline" @click="confirmMemoryRevoke">{{ $t('externalMemory.bindings.revoke') }}</t-button></div></div></template>
    </t-drawer>

    <t-dialog v-model:visible="decisionVisible" :header="decisionTitle" :confirm-btn="decisionTitle" :confirm-loading="deciding" @confirm="submitDecision"><t-form label-align="top"><t-form-item :label="$t('externalMemory.reviews.comment')" :help="decision === 'request_changes' ? $t('externalMemory.reviews.commentRequired') : $t('externalMemory.reviews.commentOptional')"><t-textarea v-model="decisionComment" :autosize="{ minRows: 3, maxRows: 6 }" /></t-form-item></t-form></t-dialog>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { MessagePlugin, type FormInstanceFunctions } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { listTenantAPIKeys, type TenantAPIKey } from '@/api/tenant'
import { listAgents, type CustomAgent } from '@/api/agent'
import { fetchAllTenantMembers, type TenantMember } from '@/api/tenant/members'
import { listDepartments, listTeams, listTeamAgents, listTeamMembers, type Department, type Team, type TeamAgent, type TeamMember } from '@/api/organization/team'
import { allowedReviewActions, bindingAssetScopes } from '@/api/external-memory/presentation'
import { createAgentBinding, getMemoryReview, listAgentBindings, listMemoryReviews, publishMemory, revokeAgentBinding, revokeMemory, reviewMemory, rotateAgentBindingKey, type AgentBinding, type CreateAgentBindingRequest, type MemoryPublication } from '@/api/external-memory'

const { t } = useI18n(); const authStore = useAuthStore(); const route = useRoute(); const router = useRouter()
const isOverview = computed(() => route.name === 'externalMemoryAdmin')
const activeTab = ref(route.name === 'externalMemoryReviews' ? 'reviews' : 'bindings'); const loading = ref(false); const reviewLoading = ref(false); const bindings = ref<AgentBinding[]>([]); const reviews = ref<MemoryPublication[]>([])
const activeBindingCount = computed(() => bindings.value.filter(item => item.status === 'active').length)
const revokedBindingCount = computed(() => bindings.value.filter(item => item.status === 'revoked').length)
const pendingReviewCount = computed(() => reviews.value.filter(item => ['pending_review', 'changes_requested'].includes(item.status)).length)
const publishedReviewCount = computed(() => reviews.value.filter(item => item.status === 'published').length)
const createVisible = ref(false); const savingBinding = ref(false); const secretVisible = ref(false); const connectorSecret = ref(''); const revokeVisible = ref(false); const revoking = ref(false); const selectedBinding = ref<AgentBinding | null>(null)
const reviewVisible = ref(false); const selectedReview = ref<MemoryPublication | null>(null); const reviewStatus = ref(''); const decisionVisible = ref(false); const decision = ref<'approve' | 'reject' | 'request_changes'>('approve'); const decisionComment = ref(''); const deciding = ref(false); const assetScopesText = ref(''); const setupPrompt = ref(''); const setupExpiresAt = ref(''); const memoryRevokeVisible = ref(false)
const organizationLoading = ref(false); const selectedDepartmentId = ref(''); const departments = ref<Department[]>([]); const teams = ref<Team[]>([]); const teamMembers = ref<TeamMember[]>([]); const teamAgents = ref<TeamAgent[]>([]); const tenantMembers = ref<TenantMember[]>([]); const allAgents = ref<CustomAgent[]>([]); const apiKeys = ref<TenantAPIKey[]>([]); const apiKeysLoading = ref(false)
const emptyBinding = (): CreateAgentBindingRequest => ({ user_api_key_id: 0, team_id: '', user_id: authStore.currentUserId, agent_id: '', external_agent: 'openclaw', connector_type: 'openclaw_plugin', capability_scopes: ['memory.context', 'memory.capture', 'memory.recall'], asset_scopes: [], capture_enabled: true, recall_enabled: true, l3_wiki_enabled: false, l3_review_required: true })
const bindingForm = ref<CreateAgentBindingRequest>(emptyBinding())
const bindingFormRef = ref<FormInstanceFunctions | null>(null)
const bindingRules = { user_api_key_id: [{ required: true, message: t('externalMemory.validation.required'), trigger: 'change' }], team_id: [{ required: true, message: t('externalMemory.validation.required'), trigger: 'change' }], agent_id: [{ required: true, message: t('externalMemory.validation.required'), trigger: 'change' }], external_agent: [{ required: true, message: t('externalMemory.validation.required'), trigger: 'blur' }] }
const departmentOptions = computed(() => departments.value.map(item => ({ label: item.name, value: item.id })))
const teamOptions = computed(() => teams.value.map(item => ({ label: item.name, value: item.id })))
const agentOptions = computed(() => teamAgents.value.map(item => ({ label: allAgents.value.find(agent => agent.id === item.agent_id)?.name || item.agent_id, value: item.agent_id })))
const userOptions = computed(() => teamMembers.value.map(item => ({ label: tenantMembers.value.find(member => member.user_id === item.user_id)?.email || item.user_id, value: item.user_id })))
const userApiKeyOptions = computed(() => apiKeys.value
  .filter(key => key.user_id === bindingForm.value.user_id)
  .filter(key => !key.revoked_at && (!key.expires_at || new Date(key.expires_at).getTime() > Date.now()))
  .map(key => ({ label: `${key.name} · ${key.user_id || t('externalMemory.fields.unassignedKey', '未绑定用户')}`, value: key.id })))
const connectorOptions = computed(() => ['openclaw_plugin', 'hermes_provider', 'openai_proxy', 'anthropic_proxy', 'mcp', 'generic_sdk'].map(value => ({ label: value === 'mcp' ? 'MCP' : t(`externalMemory.connectorTypes.${value}`), value })))
const capabilityOptions = computed(() => ['memory.context', 'memory.capture', 'memory.recall', 'memory.confirm', 'memory.publish', 'knowledge.search', 'wiki.get', 'document.read', 'context.assemble'].map(value => ({ label: t(`externalMemory.capabilities.${value.replaceAll('.', '_')}`), value })))
const reviewStatusOptions = ['pending_review', 'changes_requested', 'approved', 'publishing', 'published', 'rejected', 'revoked'].map(value => ({ label: statusLabel(value), value }))
const bindingColumns = computed(() => [{ colKey: 'external_agent', title: t('externalMemory.columns.externalAgent'), ellipsis: true }, { colKey: 'connector_type', title: t('externalMemory.columns.connector') }, { colKey: 'team_id', title: t('externalMemory.columns.team'), ellipsis: true }, { colKey: 'policy', title: t('externalMemory.columns.policy'), width: 210 }, { colKey: 'status', title: t('externalMemory.columns.status'), width: 105 }, { colKey: 'actions', title: t('externalMemory.columns.actions'), width: 150 }])
const reviewColumns = computed(() => [{ colKey: 'title', title: t('externalMemory.columns.title'), ellipsis: true }, { colKey: 'team_id', title: t('externalMemory.columns.team'), ellipsis: true }, { colKey: 'agent_id', title: t('externalMemory.columns.agent'), ellipsis: true }, { colKey: 'status', title: t('externalMemory.columns.status'), width: 125 }, { colKey: 'actions', title: t('externalMemory.columns.actions'), width: 90 }])
const canReadBindings = computed(() => authStore.canAction('external_memory', 'binding.read'))
const canCreateBinding = computed(() => authStore.canAction('external_memory', 'binding.create'))
const canRotateBinding = computed(() => authStore.canAction('external_memory', 'binding.rotate'))
const canRevokeBinding = computed(() => authStore.canAction('external_memory', 'binding.revoke'))
const canReadMemory = computed(() => authStore.canAction('external_memory', 'memory.read'))
const canReviewPermission = computed(() => authStore.canAction('external_memory', 'memory.review'))
const canPublishPermission = computed(() => authStore.canAction('external_memory', 'memory.publish'))
const canRevokeMemory = computed(() => !!selectedReview.value && authStore.canAction('external_memory', 'memory.revoke') && selectedReview.value.status === 'published')
const canReview = computed(() => canReviewPermission.value && !!selectedReview.value && allowedReviewActions(selectedReview.value.status).some(action => action !== 'publish')); const canPublish = computed(() => canPublishPermission.value && !!selectedReview.value && allowedReviewActions(selectedReview.value.status).includes('publish')); const decisionTitle = computed(() => t(`externalMemory.actions.${decision.value === 'request_changes' ? 'requestChanges' : decision.value}`))
function statusLabel(status: string) {
  if (status === 'pending_setup') return navigator.language.toLowerCase().startsWith('zh') ? '待接入' : 'Pending setup'
  return t(`externalMemory.status.${status}`, status)
}
function goTo(section: 'bindings' | 'reviews') { router.push(`/platform/external-memory/${section}`) }
function goToCreate() { router.push({ name: 'externalMemoryBindings', query: { create: '1' } }) }
function reviewTheme(status: string): 'success' | 'warning' | 'danger' | 'primary' | 'default' { if (status === 'approved' || status === 'published') return 'success'; if (status === 'pending_review' || status === 'changes_requested' || status === 'publishing') return 'warning'; if (status === 'rejected' || status === 'revoked') return 'danger'; return 'default' }
async function loadBindings() { loading.value = true; try { bindings.value = await listAgentBindings() } catch (error: any) { MessagePlugin.error(error?.message || t('externalMemory.error.load')) } finally { loading.value = false } }
async function loadReviews() { reviewLoading.value = true; try { reviews.value = await listMemoryReviews(reviewStatus.value || undefined) } catch (error: any) { MessagePlugin.error(error?.message || t('externalMemory.error.load')) } finally { reviewLoading.value = false } }
function reload() { loadBindings(); loadReviews() }
async function loadBindingOrganization() {
  organizationLoading.value = true
  apiKeysLoading.value = true
  try {
    const [departmentResponse, memberList, agentResponse, keyResponse] = await Promise.all([
      listDepartments(),
      authStore.currentTenantId ? fetchAllTenantMembers(Number(authStore.currentTenantId)) : Promise.resolve([]),
      listAgents({ creator: 'all' }),
      authStore.currentTenantId ? listTenantAPIKeys(Number(authStore.currentTenantId)) : Promise.resolve({ data: [] }),
    ])
    departments.value = departmentResponse.data || []
    tenantMembers.value = memberList
    allAgents.value = (agentResponse as any)?.data || []
    apiKeys.value = keyResponse.data || []
    if (departments.value.length > 0) {
      selectedDepartmentId.value = selectedDepartmentId.value || departments.value[0].id
      await loadBindingTeams(selectedDepartmentId.value)
    } else {
      teams.value = []
      teamMembers.value = []
      teamAgents.value = []
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('externalMemory.fields.organizationLoadFailed'))
  } finally { organizationLoading.value = false; apiKeysLoading.value = false }
}
async function loadBindingTeams(departmentId: string) {
  if (!departmentId) { teams.value = []; return }
  const response = await listTeams(departmentId)
  teams.value = response.data || []
  if (!teams.value.some(team => team.id === bindingForm.value.team_id)) bindingForm.value.team_id = ''
  teamMembers.value = []
  teamAgents.value = []
}
async function loadBindingTeamResources(teamId: string) {
  if (!teamId) { teamMembers.value = []; teamAgents.value = []; bindingForm.value.agent_id = ''; bindingForm.value.user_id = ''; return }
  const [memberResponse, agentResponse] = await Promise.all([listTeamMembers(teamId), listTeamAgents(teamId)])
  teamMembers.value = memberResponse.data || []
  teamAgents.value = agentResponse.data || []
  if (!teamAgents.value.some(agent => agent.agent_id === bindingForm.value.agent_id)) bindingForm.value.agent_id = ''
  if (!teamMembers.value.some(member => member.user_id === bindingForm.value.user_id)) bindingForm.value.user_id = ''
}
async function openCreate() { bindingForm.value = emptyBinding(); selectedDepartmentId.value = ''; teams.value = []; teamMembers.value = []; teamAgents.value = []; assetScopesText.value = ''; await loadBindingOrganization(); createVisible.value = true }
async function submitBindingForm() { if (savingBinding.value) return; const valid = await bindingFormRef.value?.validate(); if (valid !== true) return; await createBinding() }
async function createBinding() { savingBinding.value = true; try { if (!bindingForm.value.user_api_key_id || !selectedDepartmentId.value || !bindingForm.value.team_id || !bindingForm.value.agent_id || !bindingForm.value.user_id) { MessagePlugin.warning(t('externalMemory.fields.selectOrganizationFirst')); return } const capabilityScopes = [...bindingForm.value.capability_scopes]; if (bindingForm.value.l3_wiki_enabled && !capabilityScopes.includes('memory.publish')) capabilityScopes.push('memory.publish'); const assetScopes = bindingAssetScopes(bindingForm.value.team_id, assetScopesText.value.split(/[\n,]/)); const result = await createAgentBinding({ ...bindingForm.value, team_id: bindingForm.value.team_id.trim(), agent_id: bindingForm.value.agent_id.trim(), external_agent: bindingForm.value.external_agent.trim(), capability_scopes: capabilityScopes, asset_scopes: assetScopes, l3_review_required: bindingForm.value.l3_wiki_enabled || bindingForm.value.l3_review_required }); createVisible.value = false; connectorSecret.value = result.connector_secret; setupPrompt.value = result.setup_prompt || ''; setupExpiresAt.value = result.setup_expires_at || ''; secretVisible.value = true; await loadBindings() } catch (error: any) { const message = String(error?.message || ''); MessagePlugin.error(message.toLowerCase().includes('invalid agent binding') ? t('externalMemory.bindingError') : (message || t('externalMemory.error.save'))) } finally { savingBinding.value = false } }
function closeSecret() { connectorSecret.value = ''; setupPrompt.value = ''; setupExpiresAt.value = ''; secretVisible.value = false }
async function copySecret() { try { await navigator.clipboard.writeText(connectorSecret.value); MessagePlugin.success(t('externalMemory.secret.copied')) } catch { MessagePlugin.warning(t('externalMemory.secret.copyFailed')) } }
async function copyPrompt() { try { await navigator.clipboard.writeText(setupPrompt.value); MessagePlugin.success(t('externalMemory.setupWizard.copied')) } catch { MessagePlugin.warning(t('externalMemory.secret.copyFailed')) } }
function confirmRevoke(binding: AgentBinding) { selectedBinding.value = binding; revokeVisible.value = true }
function confirmMemoryRevoke() { decisionComment.value = ''; memoryRevokeVisible.value = true }
async function revoke() { if (!selectedBinding.value) return; revoking.value = true; try { await revokeAgentBinding(selectedBinding.value.id); revokeVisible.value = false; MessagePlugin.success(t('externalMemory.bindings.revoked')); await loadBindings() } catch (error: any) { MessagePlugin.error(error?.message || t('externalMemory.error.save')) } finally { revoking.value = false } }
async function rotate(binding: AgentBinding) { try { const result = await rotateAgentBindingKey(binding.id); connectorSecret.value = result.connector_secret; setupPrompt.value = result.setup_prompt || ''; setupExpiresAt.value = result.setup_expires_at || ''; secretVisible.value = true; MessagePlugin.success(t('externalMemory.bindings.rotated')) } catch (error: any) { MessagePlugin.error(error?.message || t('externalMemory.error.save')) } }
async function openReview(review: MemoryPublication) { try { selectedReview.value = (await getMemoryReview(review.id)).publication; reviewVisible.value = true } catch (error: any) { MessagePlugin.error(error?.message || t('externalMemory.error.load')) } }
function openDecision(next: 'approve' | 'reject' | 'request_changes') { decision.value = next; decisionComment.value = ''; decisionVisible.value = true }
async function submitDecision() { if (!selectedReview.value) return; if (decision.value === 'request_changes' && !decisionComment.value.trim()) { MessagePlugin.warning(t('externalMemory.reviews.commentRequired')); return } deciding.value = true; try { await reviewMemory(selectedReview.value.id, decision.value, decisionComment.value.trim()); decisionVisible.value = false; MessagePlugin.success(t('externalMemory.actions.saved')); await refreshSelectedReview() } catch (error: any) { MessagePlugin.error(error?.message || t('externalMemory.error.save')) } finally { deciding.value = false } }
async function publish() { if (!selectedReview.value) return; deciding.value = true; try { await publishMemory(selectedReview.value.id); MessagePlugin.success(t('externalMemory.actions.published')); await refreshSelectedReview() } catch (error: any) { MessagePlugin.error(error?.message || t('externalMemory.error.save')) } finally { deciding.value = false } }
async function revokeMemoryProjection() { if (!selectedReview.value) return; deciding.value = true; try { await revokeMemory(selectedReview.value.id, decisionComment.value.trim()); memoryRevokeVisible.value = false; MessagePlugin.success(t('externalMemory.bindings.revoked')); await refreshSelectedReview() } catch (error: any) { MessagePlugin.error(error?.message || t('externalMemory.error.save')) } finally { deciding.value = false } }
async function refreshSelectedReview() { if (!selectedReview.value) return; selectedReview.value = (await getMemoryReview(selectedReview.value.id)).publication; await loadReviews() }
watch(() => bindingForm.value.l3_wiki_enabled, enabled => { if (enabled) bindingForm.value.l3_review_required = true })
watch(() => bindingForm.value.user_id, userId => {
  if (!userId) { bindingForm.value.user_api_key_id = 0; return }
  const selected = apiKeys.value.find(key => key.id === bindingForm.value.user_api_key_id)
  if (selected && selected.user_id !== userId) bindingForm.value.user_api_key_id = 0
})
watch(selectedDepartmentId, departmentId => { if (createVisible.value) void loadBindingTeams(departmentId) })
watch(() => bindingForm.value.team_id, teamId => { if (createVisible.value) void loadBindingTeamResources(teamId) })
watch(() => route.name, name => {
  if (name === 'externalMemoryBindings') activeTab.value = 'bindings'
  if (name === 'externalMemoryReviews') activeTab.value = 'reviews'
})
watch(() => route.query.create, create => {
  if (route.name === 'externalMemoryBindings' && create === '1' && !createVisible.value) {
    openCreate()
    router.replace({ name: 'externalMemoryBindings' })
  }
})
onMounted(() => {
  const tab = route.query.tab
  if (route.name === 'externalMemoryAdmin' && (tab === 'bindings' || tab === 'reviews')) {
    router.replace(`/platform/external-memory/${tab}`)
    return
  }
  if (route.name === 'externalMemoryBindings' && route.query.create === '1') {
    openCreate()
    router.replace({ name: 'externalMemoryBindings' })
  }
  reload()
})
</script>

<style scoped lang="less">
.memory-admin { max-width: 1440px; margin: 0 auto; padding: 28px clamp(16px, 3vw, 40px) 48px; color: var(--td-text-color-primary); }
.memory-overview { display:flex; flex-direction:column; gap:24px; }
.overview-grid { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:16px; }
.overview-card { display:flex; flex-direction:column; gap:7px; padding:20px; border:1px solid var(--td-component-stroke); border-radius:12px; background:var(--td-bg-color-container); box-shadow:0 6px 18px rgb(31 35 40 / 5%); }
.overview-card span { color:var(--td-text-color-secondary); font-size:13px; }.overview-card strong { color:var(--td-brand-color); font-size:32px; line-height:1.1; }.overview-card small { color:var(--td-text-color-secondary); }.overview-card--warning strong { color:var(--td-warning-color); }.overview-card--success strong { color:var(--td-success-color); }
.overview-actions { display:flex; align-items:center; justify-content:space-between; gap:20px; padding:22px 24px; border:1px solid var(--td-component-stroke); border-radius:12px; background:var(--td-bg-color-container); }.overview-actions h2 { margin:0; font-size:20px; }.overview-actions p { margin:7px 0 0; color:var(--td-text-color-secondary); }.overview-action-buttons { display:flex; gap:10px; flex-wrap:wrap; }
.memory-admin__header { display:flex; align-items:flex-start; justify-content:space-between; gap:24px; margin-bottom:18px; } .memory-admin__header h1 { margin:0; font-size:28px; line-height:1.25; } .memory-admin__header p { margin:8px 0 0; color:var(--td-text-color-secondary); line-height:1.6; } .memory-admin__eyebrow { color:var(--td-brand-color)!important; font-weight:600; font-size:13px; letter-spacing:.04em; }
.memory-admin__notice { margin-bottom:20px; } .memory-admin__tabs { background:var(--td-bg-color-container); border:1px solid var(--td-component-stroke); border-radius:12px; padding:0 20px 20px; } .memory-admin__section { padding-top:20px; } .section-heading { display:flex; align-items:flex-start; justify-content:space-between; gap:20px; margin-bottom:20px; } .section-heading h2 { margin:0; font-size:18px; } .section-heading p { margin:6px 0 0; color:var(--td-text-color-secondary); font-size:14px; } .section-heading--review .t-select { width:190px; }
.policy-tags,.table-actions,.review-actions { display:flex; flex-wrap:wrap; align-items:center; gap:6px; } .form-grid { display:grid; grid-template-columns:1fr 1fr; gap:0 16px; } .switch-grid { display:flex; flex-wrap:wrap; gap:16px 28px; padding:4px 0 12px; } .secret-box { display:flex; align-items:center; gap:8px; margin-top:16px; padding:12px; border:1px solid var(--td-component-stroke); background:var(--td-bg-color-secondarycontainer); border-radius:8px; } .secret-box code { flex:1; min-width:0; overflow-wrap:anywhere; font-family:var(--app-font-family-mono, monospace); font-size:12px; }
.binding-capabilities :deep(.t-checkbox-group) { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:10px 18px; }
.setup-wizard__status { display:flex; align-items:center; gap:12px; margin:16px 0 8px; color:var(--td-text-color-secondary); }
.setup-wizard__hint { color:var(--td-text-color-secondary); line-height:1.6; }
.setup-prompt-box { position:relative; border:1px solid var(--td-component-stroke); border-radius:8px; background:var(--td-bg-color-secondarycontainer); padding:14px; }
.setup-prompt-box pre { max-height:420px; overflow:auto; white-space:pre-wrap; word-break:break-word; margin:0 0 12px; font:12px/1.65 var(--app-font-family-mono,monospace); }
:deep(.binding-dialog) { max-width:calc(100vw - 32px); }
:deep(.binding-dialog .t-dialog__body) { max-height:min(68vh, 720px); overflow-y:auto; }
:deep(.binding-dialog .t-dialog__footer) { position:sticky; bottom:0; z-index:1; background:var(--td-bg-color-container); }
.review-detail { display:flex; flex-direction:column; gap:22px; } .review-detail__top { display:flex; justify-content:space-between; gap:16px; } .review-detail h2 { margin:0; font-size:21px; } .review-detail h3 { margin:0 0 8px; font-size:15px; } .review-detail p { margin:6px 0 0; color:var(--td-text-color-secondary); } .review-meta { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:12px; margin:0; } .review-meta div { padding:12px; background:var(--td-bg-color-secondarycontainer); border-radius:8px; } .review-meta dt { font-size:12px; color:var(--td-text-color-secondary); } .review-meta dd { margin:4px 0 0; overflow-wrap:anywhere; } .markdown-preview { margin:0; max-height:360px; overflow:auto; padding:14px; white-space:pre-wrap; background:var(--td-bg-color-secondarycontainer); border:1px solid var(--td-component-stroke); border-radius:8px; font-family:var(--app-font-family-mono,monospace); font-size:13px; line-height:1.65; } .evidence-list { margin:0; padding-left:20px; word-break:break-word; line-height:1.7; } .muted { color:var(--td-text-color-secondary); }
@media (max-width: 900px) { .overview-grid { grid-template-columns:repeat(2,minmax(0,1fr)); } .overview-actions { align-items:flex-start; flex-direction:column; } }
@media (max-width: 700px) { .memory-admin { padding:20px 16px 32px; } .memory-admin__header,.section-heading { flex-direction:column; } .section-heading--review .t-select { width:100%; } .memory-admin__tabs { padding:0 12px 16px; } .form-grid,.review-meta,.overview-grid { grid-template-columns:1fr; } .binding-capabilities :deep(.t-checkbox-group) { grid-template-columns:1fr; } .table-actions { flex-wrap:nowrap; } }
</style>
