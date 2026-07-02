package defaults

// HelperScript returns the JS source for Handlebars.js helpers required
// by these templates. Inject this into the chromedp page (or your Node
// render process) before compiling/executing any template — it must run
// once per fresh page/process since there's no persistent Handlebars
// registry across chromedp page loads.
const HelperScript = `
Handlebars.registerHelper('coalesce', function (...args) {
  const values = args.slice(0, -1); // drop trailing Handlebars options object
  return values.find(v => v !== undefined && v !== null && v !== '') || '';
});
`
