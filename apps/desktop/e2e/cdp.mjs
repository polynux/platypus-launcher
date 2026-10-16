// CDP driver (issue #69): the smallest thing that drives the sandbox app.
// Node's built-in WebSocket and fetch only, no new dependency. It connects
// to the sandbox's remote-debugging port, evaluates JS in the main window,
// waits for text, clicks, types and screenshots to PNG.
export class Cdp {
  constructor(url, ws) {
    this.url = url;
    this.ws = ws;
    this.nextId = 0;
    this.pending = new Map();
    ws.onmessage = (ev) => {
      let m;
      try { m = JSON.parse(ev.data); } catch { return; }
      if (m.id != null && this.pending.has(m.id)) {
        const p = this.pending.get(m.id);
        this.pending.delete(m.id);
        if (m.error) p.reject(new Error(`CDP ${m.method ?? ''}: ${JSON.stringify(m.error)}`));
        else p.resolve(m.result);
      }
    };
  }

  static async connect(debugPort, { timeoutMs = 60000 } = {}) {
    const end = Date.now() + timeoutMs;
    let lastErr;
    while (Date.now() < end) {
      try {
        const res = await fetch(`http://127.0.0.1:${debugPort}/json/list`);
        if (res.ok) {
          const targets = await res.json();
          const pages = targets.filter((t) => t.type === 'page' && t.webSocketDebuggerUrl);
          // The main window is the app page (app:// or the Vite dev URL);
          // DevTools and background pages never match first.
          const pick = pages.find((t) => (t.url ?? '').includes('app://'))
            || pages.find((t) => /127\.0\.0\.1:\d+/.test(t.url ?? ''))
            || pages[0];
          if (pick) {
            const ws = new WebSocket(pick.webSocketDebuggerUrl);
            await new Promise((resolve, reject) => {
              ws.onopen = () => resolve();
              ws.onerror = (e) => reject(new Error(`CDP websocket failed: ${e?.message ?? e}`));
            });
            const cdp = new Cdp(pick.url, ws);
            try {
              await cdp.send('Page.enable', {});
              await cdp.send('Runtime.enable', {});
            } catch {}
            return cdp;
          }
        }
      } catch (e) { lastErr = e; }
      await new Promise((r) => setTimeout(r, 500));
    }
    throw new Error(`CDP: no debuggable page on :${debugPort} (${lastErr?.message ?? 'empty list'})`);
  }

  send(method, params = {}) {
    const id = ++this.nextId;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.ws.send(JSON.stringify({ id, method, params }));
      setTimeout(() => {
        if (this.pending.has(id)) {
          this.pending.delete(id);
          reject(new Error(`CDP timeout: ${method}`));
        }
      }, 30000);
    });
  }

  // Evaluate a function body in the page and return its JSON value. The fn
  // runs as `(...args) => result`; set awaitPromise when it returns one.
  async evaluate(fn, args = [], { awaitPromise = true } = {}) {
    const expression = `(${fn})(${args.map((a) => JSON.stringify(a)).join(',')})`;
    const r = await this.send('Runtime.evaluate', {
      expression,
      awaitPromise,
      returnByValue: true,
      userGesture: true,
    });
    if (r.exceptionDetails) {
      const t = r.exceptionDetails.exception?.description
        ?? r.exceptionDetails.text ?? JSON.stringify(r.exceptionDetails);
      throw new Error(`evaluate threw: ${t}\n  in: ${expression.slice(0, 300)}`);
    }
    return r.result?.value;
  }

  async waitFor(fn, args = [], { timeoutMs = 30000, pollMs = 500, label = 'condition' } = {}) {
    const end = Date.now() + timeoutMs;
    let last;
    while (Date.now() < end) {
      try {
        last = await this.evaluate(fn, args);
        if (last) return last;
      } catch (e) { last = e; }
      await new Promise((r) => setTimeout(r, pollMs));
    }
    throw new Error(`CDP waitFor timed out (${label}): ${last instanceof Error ? last.message : JSON.stringify(last)?.slice(0, 200)}`);
  }

  waitForText(text, opts = {}) {
    // innerText respects CSS text-transform (headings render UPPERCASE), so
    // match case-insensitively: steps name the words users read.
    return this.waitFor(
      (t) => (document.body?.innerText ?? '').toLowerCase().includes(t.toLowerCase()) ? true : false,
      [text],
      { label: `text ${JSON.stringify(text)}`, ...opts },
    );
  }

  // Click the first element matching a CSS selector (scrolls it into view).
  async click(selector, { timeoutMs = 30000 } = {}) {
    await this.waitFor(
      (s) => { const el = document.querySelector(s); return el ? true : false; },
      [selector],
      { timeoutMs, label: `selector ${selector}` },
    );
    const ok = await this.evaluate((s) => {
      const el = document.querySelector(s);
      if (!el) return false;
      el.scrollIntoView({ block: 'center' });
      el.click();
      return true;
    }, [selector]);
    if (!ok) throw new Error(`CDP click missed: ${selector}`);
  }

  // Click by visible text (buttons, links, options): the first element whose
  // text contains it. Prefer this over selectors where the UI has stable words.
  // Matches case-insensitively like waitForText (CSS can uppercase headings).
  async clickText(text, { timeoutMs = 30000 } = {}) {
    await this.waitForText(text, { timeoutMs });
    const ok = await this.evaluate((t) => {
      const low = t.toLowerCase();
      const els = [...document.querySelectorAll('button, a, [role="option"], [role="tab"], input[type="checkbox"] + span, label')];
      const hit = els.find((e) => (e.innerText ?? e.textContent ?? '').toLowerCase().includes(low));
      if (!hit) {
        // Fall back to any element with the text (radix items render as divs).
        const all = [...document.querySelectorAll('*')].find((e) =>
          e.childElementCount === 0 && (e.textContent ?? '').trim().toLowerCase() === low);
        if (!all) return false;
        all.click();
        return true;
      }
      // A label span inside a checkbox row: click the row's input instead.
      const row = hit.closest('label');
      const box = row?.querySelector('input[type="checkbox"]');
      (box ?? hit).click();
      return true;
    }, [text]);
    if (!ok) throw new Error(`CDP clickText missed: ${JSON.stringify(text)}`);
  }

  // Type into an input/textarea matching a selector: focuses, replaces the
  // value and fires the events React listens for.
  async type(selector, text) {
    await this.click(selector);
    const ok = await this.evaluate((s, v) => {
      const el = document.querySelector(s);
      if (!el) return false;
      el.focus();
      const setter = Object.getOwnPropertyDescriptor(
        el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype, 'value')?.set
        ?? Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'value')?.set;
      // Fall back to a direct set when the prototype chain hides it.
      if (setter) setter.call(el, v);
      else el.value = v;
      el.dispatchEvent(new Event('input', { bubbles: true }));
      el.dispatchEvent(new Event('change', { bubbles: true }));
      return true;
    }, [selector, text]);
    if (!ok) throw new Error(`CDP type missed: ${selector}`);
  }

  async screenshot(path) {
    const { writeFileSync, mkdirSync } = await import('node:fs');
    const { dirname } = await import('node:path');
    const r = await this.send('Page.captureScreenshot', { format: 'png' });
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, Buffer.from(r.data, 'base64'));
    return path;
  }

  close() {
    try { this.ws.close(); } catch {}
  }
}
