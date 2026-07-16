App({
  onLaunch() {
    const settings = wx.getStorageSync("keystone_settings");
    if (!settings) {
      wx.setStorageSync("keystone_settings", {
        baseUrl: "http://localhost:8080",
        apiKey: "",
        selectedKnowledgeBaseId: ""
      });
    }
  }
});
