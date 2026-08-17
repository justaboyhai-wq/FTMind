App({
  onLaunch() {
    const settings = wx.getStorageSync("fmind_settings");
    if (!settings) {
      wx.setStorageSync("fmind_settings", {
        baseUrl: "http://localhost:8080",
        apiKey: "",
        selectedKnowledgeBaseId: ""
      });
    }
  }
});
