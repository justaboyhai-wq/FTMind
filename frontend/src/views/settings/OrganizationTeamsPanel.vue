<template>
  <section class="organization-teams-panel">
    <div class="org-panel-head">
      <div>
        <h3>{{ t('tenantMember.organization.title') }}</h3>
        <p>{{ t('tenantMember.organization.description') }}</p>
      </div>
      <div class="org-panel-actions" v-if="canManage">
        <t-button variant="outline" @click="departmentDialog = true">{{ t('tenantMember.organization.createDepartment') }}</t-button>
        <t-button theme="primary" :disabled="departments.length === 0" @click="openCreateTeam">{{ t('tenantMember.organization.createTeam') }}</t-button>
      </div>
    </div>

    <t-alert v-if="!loading && departments.length === 0" theme="info"
      :message="t('tenantMember.organization.empty')" />
    <div v-else class="org-columns">
      <div class="org-column org-departments">
        <div class="org-column-title">{{ t('tenantMember.organization.departments') }}</div>
        <button v-for="department in departments" :key="department.id" type="button"
          :class="['org-list-item', { active: department.id === selectedDepartmentId }]"
          @click="selectDepartment(department.id)">
          <span>{{ department.name }}</span>
          <small>{{ department.code }}</small>
        </button>
      </div>
      <div class="org-column org-teams">
        <div class="org-column-title">{{ t('tenantMember.organization.teams') }}</div>
        <div v-if="teams.length === 0" class="org-empty">{{ t('tenantMember.organization.noTeams') }}</div>
        <button v-for="team in teams" :key="team.id" type="button"
          :class="['org-list-item', { active: team.id === selectedTeamId }]"
          @click="selectTeam(team.id)">
          <span>{{ team.name }}</span>
          <small>{{ team.code }}</small>
        </button>
      </div>
      <div class="org-column org-detail">
        <template v-if="selectedTeam">
          <div class="org-detail-head">
            <div>
              <strong>{{ selectedTeam.name }}</strong>
              <span>{{ selectedTeam.code }}</span>
            </div>
            <t-button v-if="canManage" theme="danger" variant="text" size="small" @click="removeSelectedTeam">
              {{ t('tenantMember.organization.deleteTeam') }}
            </t-button>
          </div>
          <div class="org-detail-section">
            <div class="org-detail-title">
              <span>{{ t('tenantMember.organization.members') }}</span>
              <t-select v-if="canManage" v-model="memberToAdd" :options="memberOptions" clearable
                :placeholder="t('tenantMember.organization.addMember')" @change="addMember" />
            </div>
            <div v-if="teamMembers.length === 0" class="org-empty">{{ t('tenantMember.organization.noMembers') }}</div>
            <div v-for="member in teamMembers" :key="member.user_id" class="org-resource-row">
              <span>{{ memberLabel(member.user_id) }}</span>
              <t-button v-if="canManage" theme="danger" variant="text" size="small" @click="removeMember(member.user_id)">
                {{ t('common.remove') }}
              </t-button>
            </div>
          </div>
          <div class="org-detail-section">
            <div class="org-detail-title">
              <span>{{ t('tenantMember.organization.agents') }}</span>
              <t-select v-if="canManage" v-model="agentToAdd" :options="agentOptions" :disabled="agentOptions.length === 0" clearable
                :placeholder="t('tenantMember.organization.addAgent')" @change="addAgent" />
            </div>
            <div v-if="canManage && bindableAgents.length === 0" class="org-hint">
              {{ t('tenantMember.organization.noBindableAgents') }}
            </div>
            <div v-if="teamAgents.length === 0" class="org-empty">{{ t('tenantMember.organization.noAgents') }}</div>
            <div v-for="agent in teamAgents" :key="agent.agent_id" class="org-resource-row">
              <span>{{ agentLabel(agent.agent_id) }}</span>
              <t-button v-if="canManage" theme="danger" variant="text" size="small" @click="removeAgent(agent.agent_id)">
                {{ t('common.remove') }}
              </t-button>
            </div>
          </div>
        </template>
        <div v-else class="org-empty">{{ t('tenantMember.organization.selectTeam') }}</div>
      </div>
    </div>

    <t-dialog v-model:visible="departmentDialog" :header="t('tenantMember.organization.createDepartment')"
      :confirm-btn="{ content: t('common.confirm'), loading: saving }" @confirm="createDepartmentAction">
      <t-form :data="departmentForm" label-align="top">
        <t-form-item :label="t('tenantMember.organization.name')"><t-input v-model="departmentForm.name" /></t-form-item>
        <t-form-item :label="t('tenantMember.organization.code')"><t-input v-model="departmentForm.code" /></t-form-item>
      </t-form>
    </t-dialog>
    <t-dialog v-model:visible="teamDialog" :header="t('tenantMember.organization.createTeam')"
      :confirm-btn="{ content: t('common.confirm'), loading: saving }" @confirm="createTeamAction">
      <t-form :data="teamForm" label-align="top">
        <t-form-item :label="t('tenantMember.organization.department')"><t-select v-model="teamForm.department_id" :options="departmentOptions" /></t-form-item>
        <t-form-item :label="t('tenantMember.organization.name')"><t-input v-model="teamForm.name" /></t-form-item>
        <t-form-item :label="t('tenantMember.organization.code')"><t-input v-model="teamForm.code" /></t-form-item>
      </t-form>
    </t-dialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { listAgents, type CustomAgent } from '@/api/agent'
import { fetchAllTenantMembers, type TenantMember } from '@/api/tenant/members'
import {
  addTeamAgent, addTeamMember, createDepartment, createTeam, deleteTeam,
  listDepartments, listTeamAgents, listTeamMembers, listTeams, removeTeamAgent,
  removeTeamMember, type Department, type Team, type TeamAgent, type TeamMember,
} from '@/api/organization/team'

const { t } = useI18n()
const authStore = useAuthStore()
const canManage = computed(() => authStore.canAccessAllTenants || authStore.hasRole('admin'))
const loading = ref(false)
const saving = ref(false)
const departments = ref<Department[]>([])
const teams = ref<Team[]>([])
const members = ref<TenantMember[]>([])
const agents = ref<CustomAgent[]>([])
const teamMembers = ref<TeamMember[]>([])
const teamAgents = ref<TeamAgent[]>([])
const selectedDepartmentId = ref('')
const selectedTeamId = ref('')
const memberToAdd = ref('')
const agentToAdd = ref('')
const departmentDialog = ref(false)
const teamDialog = ref(false)
const departmentForm = ref({ name: '', code: '' })
const teamForm = ref({ department_id: '', name: '', code: '' })

const selectedTeam = computed(() => teams.value.find((team) => team.id === selectedTeamId.value))
const departmentOptions = computed(() => departments.value.map((item) => ({ label: item.name, value: item.id })))
const memberOptions = computed(() => members.value.filter((member) => !teamMembers.value.some((item) => item.user_id === member.user_id)).map((member) => ({ label: member.email || member.username || member.user_id, value: member.user_id })))
// Built-in agents (快速问答/智能推理) are global conversation presets, not
// tenant-owned resources.  The team_agents table intentionally accepts only
// tenant custom_agents, so never present built-ins as selectable options.
const bindableAgents = computed(() => agents.value.filter((agent) => {
  if (agent.is_builtin || agent.id.startsWith('builtin-')) return false
  // Fail closed when the list response does not carry an explicit tenant.
  // The API itself re-checks ownership; the UI must not offer an ambiguous
  // cross-tenant resource as a selectable option.
  return Boolean(authStore.currentTenantId)
    && Number(agent.tenant_id) === Number(authStore.currentTenantId)
}))
const agentOptions = computed(() => bindableAgents.value
  .filter((agent) => !teamAgents.value.some((item) => item.agent_id === agent.id))
  .map((agent) => ({ label: agent.name || agent.id, value: agent.id })))

function memberLabel(id: string) { const item = members.value.find((member) => member.user_id === id); return item?.email || item?.username || id }
function agentLabel(id: string) { return agents.value.find((agent) => agent.id === id)?.name || id }
function organizationError(error: any, fallbackKey: string) {
  const status = Number(error?.status || error?.response?.status)
  const raw = String(error?.message || error?.error?.message || '').toLowerCase()
  if (status === 409 || raw.includes('duplicate') || raw.includes('already exists') || raw.includes('conflict')) {
    return t('tenantMember.organization.conflict')
  }
  if (raw.includes('invalid organization resource') || raw.includes('organization resource')) {
    return t('tenantMember.organization.invalidResource')
  }
  return error?.message || t(fallbackKey)
}

async function loadBase() {
  loading.value = true
  try {
    const [departmentResponse, memberList, agentResponse] = await Promise.all([
      listDepartments(),
      authStore.currentTenantId ? fetchAllTenantMembers(Number(authStore.currentTenantId)) : Promise.resolve([]),
      listAgents({ creator: 'all' }),
    ])
    departments.value = departmentResponse.data || []
    members.value = memberList
    agents.value = (agentResponse as any)?.data || []
    if (departments.value.length > 0) await selectDepartment(departments.value[0].id)
  } catch (error: any) {
    MessagePlugin.error(organizationError(error, 'tenantMember.organization.loadFailed'))
  } finally { loading.value = false }
}
async function selectDepartment(id: string) {
  selectedDepartmentId.value = id
  selectedTeamId.value = ''
  const response = await listTeams(id)
  teams.value = response.data || []
  if (teams.value.length > 0) await selectTeam(teams.value[0].id)
}
async function selectTeam(id: string) {
  selectedTeamId.value = id
  const [memberResponse, agentResponse] = await Promise.all([listTeamMembers(id), listTeamAgents(id)])
  teamMembers.value = memberResponse.data || []
  teamAgents.value = agentResponse.data || []
}
function openCreateTeam() { teamForm.value.department_id = selectedDepartmentId.value || departments.value[0]?.id || ''; teamDialog.value = true }
async function createDepartmentAction() {
  if (!departmentForm.value.name.trim() || !departmentForm.value.code.trim()) return
  saving.value = true
  try { await createDepartment(departmentForm.value); departmentDialog.value = false; departmentForm.value = { name: '', code: '' }; await loadBase() } catch (error: any) { MessagePlugin.error(organizationError(error, 'tenantMember.organization.saveFailed')) } finally { saving.value = false }
}
async function createTeamAction() {
  if (!teamForm.value.department_id || !teamForm.value.name.trim() || !teamForm.value.code.trim()) return
  saving.value = true
  try { await createTeam(teamForm.value); teamDialog.value = false; await selectDepartment(teamForm.value.department_id) } catch (error: any) { MessagePlugin.error(organizationError(error, 'tenantMember.organization.saveFailed')) } finally { saving.value = false }
}
async function removeSelectedTeam() {
  if (!selectedTeam.value) return
  const team = selectedTeam.value
  const dialog = DialogPlugin.confirm({ header: t('tenantMember.organization.deleteTeam'), body: t('tenantMember.organization.deleteConfirm', { name: team.name }), confirmBtn: t('common.confirm'), cancelBtn: t('common.cancel'), onConfirm: async () => { try { await deleteTeam(team.id); selectedTeamId.value = ''; await selectDepartment(selectedDepartmentId.value); dialog.hide() } catch (error: any) { MessagePlugin.error(organizationError(error, 'tenantMember.organization.deleteFailed')) } } })
}
async function addMember(id: string) { if (!id || !selectedTeamId.value) return; try { await addTeamMember(selectedTeamId.value, id); memberToAdd.value = ''; await selectTeam(selectedTeamId.value) } catch (error: any) { MessagePlugin.error(organizationError(error, 'tenantMember.organization.saveFailed')) } }
async function removeMember(id: string) { if (!selectedTeamId.value) return; try { await removeTeamMember(selectedTeamId.value, id); await selectTeam(selectedTeamId.value) } catch (error: any) { MessagePlugin.error(organizationError(error, 'tenantMember.organization.saveFailed')) } }
async function addAgent(id: string) { if (!id || !selectedTeamId.value) return; try { await addTeamAgent(selectedTeamId.value, id); agentToAdd.value = ''; await selectTeam(selectedTeamId.value) } catch (error: any) { MessagePlugin.error(organizationError(error, 'tenantMember.organization.saveFailed')) } }
async function removeAgent(id: string) { if (!selectedTeamId.value) return; try { await removeTeamAgent(selectedTeamId.value, id); await selectTeam(selectedTeamId.value) } catch (error: any) { MessagePlugin.error(organizationError(error, 'tenantMember.organization.saveFailed')) } }

onMounted(loadBase)
</script>

<style scoped>
.organization-teams-panel { display: flex; flex-direction: column; gap: 16px; }
.org-panel-head, .org-detail-head, .org-detail-title { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.org-panel-head h3 { margin: 0 0 6px; }
.org-panel-head p { margin: 0; color: var(--td-text-color-secondary); }
.org-panel-actions { display: flex; gap: 8px; }
.org-columns { display: grid; grid-template-columns: 0.8fr 1fr 1.5fr; gap: 12px; min-height: 360px; }
.org-column { border: 1px solid var(--td-component-border); border-radius: 8px; padding: 12px; }
.org-column-title { font-weight: 600; margin-bottom: 10px; }
.org-list-item { display: flex; justify-content: space-between; width: 100%; border: 0; background: transparent; padding: 10px; border-radius: 6px; text-align: left; cursor: pointer; }
.org-list-item:hover, .org-list-item.active { background: var(--td-brand-color-light); color: var(--td-brand-color); }
.org-list-item small { color: var(--td-text-color-secondary); }
.org-detail-head { border-bottom: 1px solid var(--td-component-border); padding-bottom: 12px; }
.org-detail-head strong, .org-detail-head span { display: block; }
.org-detail-head span { color: var(--td-text-color-secondary); font-size: 12px; margin-top: 4px; }
.org-detail-section { padding-top: 16px; }
.org-detail-title { margin-bottom: 8px; font-weight: 600; }
.org-detail-title .t-select { width: 190px; }
.org-resource-row { display: flex; justify-content: space-between; align-items: center; padding: 7px 0; border-bottom: 1px solid var(--td-component-border); }
.org-empty { color: var(--td-text-color-secondary); padding: 24px 8px; text-align: center; }
.org-hint { color: var(--td-text-color-secondary); font-size: 12px; margin: -2px 0 8px; }
@media (max-width: 900px) { .org-columns { grid-template-columns: 1fr; } }
</style>
