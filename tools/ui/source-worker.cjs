const hljs = require("highlight.js/lib/core");
hljs.registerLanguage("go", require("highlight.js/lib/languages/go"));
hljs.registerLanguage("typescript", require("highlight.js/lib/languages/typescript"));
hljs.registerLanguage("javascript", require("highlight.js/lib/languages/javascript"));
hljs.registerLanguage("python", require("highlight.js/lib/languages/python"));
hljs.registerLanguage("json", require("highlight.js/lib/languages/json"));
hljs.registerLanguage("yaml", require("highlight.js/lib/languages/yaml"));
hljs.registerLanguage("bash", require("highlight.js/lib/languages/bash"));
hljs.registerLanguage("xml", require("highlight.js/lib/languages/xml"));
hljs.registerLanguage("css", require("highlight.js/lib/languages/css"));
hljs.registerLanguage("markdown", require("highlight.js/lib/languages/markdown"));
hljs.registerLanguage("sql", require("highlight.js/lib/languages/sql"));
hljs.registerLanguage("ini", require("highlight.js/lib/languages/ini"));
self.onmessage = ({data}) => {
  try {
    self.postMessage({id:data.id, html:hljs.highlight(data.text,{language:data.language,ignoreIllegals:true}).value});
  } catch {
    self.postMessage({id:data.id});
  }
};
