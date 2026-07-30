package browser

import (
	"errors"

	"github.com/go-rod/rod"
)

// StealthJS is the inline JS injected on every new document of a stealth-enabled
// page. It hides the most common automation signals — navigator.webdriver,
// missing plugins/languages, headless WebGL renderer, denied notifications.
//
// This is intentionally minimal: we keep zero new dependencies. It covers
// ~90 % of trivial bot detectors. Switch to github.com/go-rod/stealth or
// nodriver if Phase 0 spike or production telemetry shows escalating
// challenge rates.
const StealthJS = `
(() => {
  Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
  Object.defineProperty(navigator, 'languages', { get: () => ['vi-VN','vi','en-US','en'] });
  Object.defineProperty(navigator, 'plugins', {
    get: () => [1,2,3,4,5].map(i => ({ name: 'Plugin '+i, filename: 'p'+i+'.so', description: '' })),
  });
  const origQuery = window.navigator.permissions && window.navigator.permissions.query;
  if (origQuery) {
    window.navigator.permissions.query = (params) =>
      params && params.name === 'notifications'
        ? Promise.resolve({ state: Notification.permission })
        : origQuery(params);
  }
  if (!window.chrome) { window.chrome = {}; }
  if (!window.chrome.runtime) { window.chrome.runtime = {}; }
  const getParameter = WebGLRenderingContext.prototype.getParameter;
  WebGLRenderingContext.prototype.getParameter = function (parameter) {
    if (parameter === 37445) return 'Intel Inc.';
    if (parameter === 37446) return 'Intel Iris OpenGL Engine';
    return getParameter.call(this, parameter);
  };
})();
`

// ApplyStealth installs StealthJS on a page so that every new document load
// (including sub-frames and SPA navigations) gets the patches before any
// site script runs. Call once per Page after creation, before Navigate.
func ApplyStealth(page *rod.Page) error {
	if page == nil {
		return errors.New("page is nil")
	}
	_, err := page.EvalOnNewDocument(StealthJS)
	return err
}
