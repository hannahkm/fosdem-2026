const marpKrokiPlugin = require('./kroki-plugin')

module.exports = {
  engine: ({ marp }) => marp.use(marpKrokiPlugin)
}

mermaid.initialize({
  securityLevel: 'loose',
  theme: 'dark',
});