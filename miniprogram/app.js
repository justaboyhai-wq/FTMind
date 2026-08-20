App({
  onLaunch() {
    const settings = wx.getStorageSync("ftmind_settings");
    if (!settings) {
      wx.setStorageSync("ftmind_settings", {
        baseUrl: "http://localhost:8080",
        apiKey: "",
        selectedKnowledgeBaseId: ""
      });
    }
  }
});
