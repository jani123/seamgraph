const { defineConfig } = require("@playwright/test");
module.exports = defineConfig({
  testDir: "tests",
  timeout: 30000,
  workers: 1,
  use: { viewport: { width: 1440, height: 1000 } },
  reporter: "line",
});
