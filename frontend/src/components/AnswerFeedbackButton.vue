<template>
  <div class="answer-feedback">
    <t-button class="answer-feedback__button" variant="text" shape="square" :class="`is-${feedback?.status || 'empty'}`" :title="buttonTitle" @click="open">
      <t-icon :name="feedback ? (feedback.status === 'resolved' ? 'check-circle' : 'error-circle-filled') : 'error-circle'" />
    </t-button>
    <t-dialog v-model:visible="visible" :header="feedback ? t('feedback.detailTitle') : t('feedback.title')" width="560px" :footer="false">
      <template v-if="feedback">
        <div class="feedback-status"><span>{{ t('feedback.status') }}</span><t-tag :theme="statusTheme">{{ statusText }}</t-tag></div>
        <p class="feedback-question">{{ feedback.question_snapshot }}</p>
        <p class="feedback-description">{{ feedback.description }}</p>
        <div v-if="feedback.public_reply" class="feedback-reply"><strong>{{ t('feedback.adminReply') }}</strong><p>{{ feedback.public_reply }}</p></div>
        <t-textarea v-if="canComment" v-model="comment" :placeholder="t('feedback.commentPlaceholder')" :maxlength="2000" :autosize="{minRows:3,maxRows:6}" />
        <div class="feedback-actions"><t-button v-if="canComment" theme="primary" :loading="saving" @click="submitComment">{{ t('feedback.addComment') }}</t-button><t-button v-if="canReopen" variant="outline" @click="reopen">{{ t('feedback.reopen') }}</t-button></div>
      </template>
      <template v-else>
        <t-form label-align="top">
          <t-form-item :label="t('feedback.categoryLabel')"><t-select v-model="form.category" :options="categories" /></t-form-item>
          <t-form-item :label="t('feedback.descriptionLabel')" required><t-textarea v-model="form.description" :placeholder="t('feedback.descriptionPlaceholder')" :maxlength="2000" :autosize="{minRows:4,maxRows:7}" /></t-form-item>
          <t-form-item :label="t('feedback.expectedLabel')"><t-textarea v-model="form.expected_correction" :maxlength="2000" :autosize="{minRows:2,maxRows:4}" /></t-form-item>
          <t-form-item :label="t('feedback.quotedLabel')"><t-textarea v-model="form.quoted_text" :maxlength="1000" :autosize="{minRows:2,maxRows:4}" /></t-form-item>
        </t-form>
        <div class="feedback-actions"><t-button variant="outline" @click="visible=false">{{ t('common.cancel') }}</t-button><t-button theme="primary" :loading="saving" @click="submit">{{ t('feedback.submit') }}</t-button></div>
      </template>
    </t-dialog>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { commentEmbedFeedback, commentFeedback, getEmbedFeedback, getMessageFeedback, listMyFeedback, reopenEmbedFeedback, reopenFeedback, submitEmbedFeedback, submitMessageFeedback, type AnswerFeedback, type FeedbackCategory } from '@/api/answer-feedback'
let minePromise: Promise<any> | null = null
const props=withDefaults(defineProps<{sessionId:string;messageId:string;embedChannelId?:string;embedToken?:string;embedSessionSig?:string;embedVisitorId?:string;question?:string;answer?:string;quotedText?:string}>(),{embedChannelId:'',embedToken:'',embedSessionSig:'',embedVisitorId:'',question:'',answer:'',quotedText:''})
const {t}=useI18n();const visible=ref(false);const saving=ref(false);const feedback=ref<AnswerFeedback|null>(null);const comment=ref('');const form=ref<{category:FeedbackCategory;description:string;expected_correction:string;quoted_text:string}>({category:'wrong_fact',description:'',expected_correction:'',quoted_text:props.quotedText||''})
const categories=computed(()=>['wrong_fact','outdated','citation_mismatch','incomplete','misunderstood','unsafe','other'].map(v=>({value:v,label:t(`feedback.categories.${v}`)})))
const isEmbed=computed(()=>Boolean(props.embedChannelId&&props.embedToken&&props.embedSessionSig));const buttonTitle=computed(()=>feedback.value?t(`feedback.statuses.${feedback.value.status}`,feedback.value.status):t('feedback.button'));const statusText=computed(()=>feedback.value?t(`feedback.statuses.${feedback.value.status}`,feedback.value.status):'');const statusTheme=computed(()=>feedback.value?.status==='resolved'?'success':feedback.value?.status==='dismissed'?'default':feedback.value?.status==='needs_info'?'warning':'primary');const canComment=computed(()=>!!feedback.value&&['pending','reviewing','needs_info','fixing'].includes(feedback.value.status));const canReopen=computed(()=>!!feedback.value&&['resolved','dismissed'].includes(feedback.value.status)&&Date.now()-new Date(feedback.value.updated_at).getTime()<7*86400000)
async function load(){try{if(isEmbed.value){const r=await getEmbedFeedback(props.embedChannelId,props.embedToken,props.sessionId,props.messageId,props.embedSessionSig,props.embedVisitorId);feedback.value=r?.data||null;return}if(!minePromise) minePromise=listMyFeedback(1,100).then(r=>r.data||[]);const all=await minePromise;feedback.value=all.find((x:any)=>x.assistant_message_id===props.messageId)||null}catch{}}
async function open(){await load();visible.value=true}
async function submit(){if(form.value.description.trim().length<10){MessagePlugin.warning(t('feedback.descriptionTooShort'));return};saving.value=true;try{const r=isEmbed.value?await submitEmbedFeedback(props.embedChannelId,props.embedToken,props.sessionId,props.messageId,props.embedSessionSig,props.embedVisitorId,form.value):await submitMessageFeedback(props.sessionId,props.messageId,form.value);feedback.value=r.data;MessagePlugin.success(t('feedback.submitted'));}catch(e:any){MessagePlugin.error(e?.message||t('feedback.submitFailed'))}finally{saving.value=false}}
async function submitComment(){if(comment.value.trim().length<2)return;saving.value=true;try{if(isEmbed.value) await commentEmbedFeedback(props.embedChannelId,props.embedToken,props.sessionId,feedback.value!.id,props.embedSessionSig,props.embedVisitorId,comment.value); else await commentFeedback(feedback.value!.id,comment.value);comment.value='';await load();MessagePlugin.success(t('feedback.commentAdded'))}finally{saving.value=false}}
async function reopen(){saving.value=true;try{const text=comment.value||t('feedback.reopenDefault');if(isEmbed.value) await reopenEmbedFeedback(props.embedChannelId,props.embedToken,props.sessionId,feedback.value!.id,props.embedSessionSig,props.embedVisitorId,text); else await reopenFeedback(feedback.value!.id,text);await load()}finally{saving.value=false}}
onMounted(load)
</script>
<style scoped lang="less">
.answer-feedback__button{color:var(--td-text-color-secondary)}.answer-feedback__button.is-pending,.answer-feedback__button.is-reviewing,.answer-feedback__button.is-fixing{color:var(--td-warning-color)}.answer-feedback__button.is-needs_info{color:var(--td-error-color)}.answer-feedback__button.is-resolved{color:var(--td-success-color)}.feedback-status{display:flex;align-items:center;gap:10px;margin-bottom:14px}.feedback-question{font-weight:600}.feedback-description{white-space:pre-wrap}.feedback-reply{padding:12px;background:var(--td-bg-color-secondarycontainer);border-radius:8px;margin:12px 0}.feedback-actions{display:flex;justify-content:flex-end;gap:8px;margin-top:16px}
</style>
