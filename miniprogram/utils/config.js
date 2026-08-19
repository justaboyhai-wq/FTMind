const STORAGE_KEY = "fmind_settings";
let runtimeApiKey = "";

function normalizeBaseUrl(baseUrl) {
  if (!baseUrl || typeof baseUrl !== "string") {
    return "";
  }

  return baseUrl.trim().replace(/\/+$/, "");
}

function getSettings() {
  const stored = wx.getStorageSync(STORAGE_KEY) || {};
  if (stored.apiKey) {
    runtimeApiKey = stored.apiKey;
    if (typeof wx.setStorageSync === "function") {
      wx.setStorageSync(STORAGE_KEY, {
        baseUrl: normalizeBaseUrl(stored.baseUrl || ""),
        selectedKnowledgeBaseId: stored.selectedKnowledgeBaseId || ""
      });
    }
  }
  return {
    baseUrl: normalizeBaseUrl(stored.baseUrl || ""),
    apiKey: runtimeApiKey,
    selectedKnowledgeBaseId: stored.selectedKnowledgeBaseId || ""
  };
}

function getPublicSettings() {
  const settings = getSettings();
  return { ...settings, apiKey: "" };
}

function saveSettings(settings) {
  const current = getSettings();
  if (Object.prototype.hasOwnProperty.call(settings, "apiKey")) {
    runtimeApiKey = settings.apiKey || "";
  }
  const next = {
    baseUrl: normalizeBaseUrl(settings.baseUrl ?? current.baseUrl),
    selectedKnowledgeBaseId:
      settings.selectedKnowledgeBaseId ?? current.selectedKnowledgeBaseId
  };
  wx.setStorageSync(STORAGE_KEY, next);
  return { ...next, apiKey: runtimeApiKey };
}

module.exports = {
  STORAGE_KEY,
  getPublicSettings,
  getSettings,
  normalizeBaseUrl,
  saveSettings
};
