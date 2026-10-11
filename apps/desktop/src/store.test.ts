import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  blockedHost,
  canOpenStore,
  explicitStoreURL,
  insideDomains,
  isLoopbackHost,
  navigationAllowed,
  popupURL,
  shouldCloseView,
  storePartition,
  stripState,
  validPluginId,
  validStoreBounds,
  type StorePluginInfo,
} from './store.ts';

const steam: StorePluginInfo = {
  id: 'steam-standalone',
  status: 'ready',
  storefront_url: 'https://store.steampowered.com/',
  storefront_domains: ['steampowered.com', 'steamcommunity.com', 'steamstatic.com'],
};

test('bounds are finite, positive, in-range rects', () => {
  assert.deepEqual(validStoreBounds({ x: 224, y: 48, width: 800.4, height: 600 }), { x: 224, y: 48, width: 800, height: 600 });
  for (const bad of [
    null, undefined, 'rect', 5, [],
    { x: 0, y: 0, width: 0, height: 600 },
    { x: 0, y: 0, width: -10, height: 600 },
    { x: -1, y: 0, width: 10, height: 10 },
    { x: 0, y: 0, width: 100000, height: 10 },
    { x: NaN, y: 0, width: 10, height: 10 },
    { x: 0, y: 0, width: Infinity, height: 10 },
    { x: 'a', y: 0, width: 10, height: 10 },
    { x: 0, y: 0, width: 10 },
  ]) {
    assert.equal(validStoreBounds(bad), null, JSON.stringify(bad));
  }
});

test('partitions are per plugin and persistent', () => {
  assert.equal(storePartition('steam-standalone'), 'persist:store-steam-standalone');
  assert.notEqual(storePartition('a'), storePartition('b'));
  assert.ok(!validPluginId('../evil') && !validPluginId('') && validPluginId('echo-b'));
});

test('explicit opens stay https and inside the domains', () => {
  assert.equal(explicitStoreURL(steam, undefined), 'https://store.steampowered.com/');
  assert.equal(explicitStoreURL(steam, 'https://store.steampowered.com/app/620/'), 'https://store.steampowered.com/app/620/');
  assert.equal(explicitStoreURL(steam, 'https://checkout.stripe.com/pay'), '');
  assert.equal(explicitStoreURL(steam, 'http://store.steampowered.com/'), '');
  assert.equal(explicitStoreURL(steam, 'steam://install/620'), '');
  assert.equal(explicitStoreURL(steam, 'javascript:alert(1)'), '');
  assert.equal(explicitStoreURL(steam, 'not a url'), '');
  assert.equal(explicitStoreURL(steam, 5), '');
  assert.equal(explicitStoreURL({ ...steam, status: 'ready', storefront_domains: [] }, 'https://store.steampowered.com/'), '');
});

test('the test plugin may open its local page', () => {
  const echo: StorePluginInfo = {
    id: 'echo', status: 'ready',
    storefront_url: 'http://127.0.0.1:18427/store/', storefront_domains: ['127.0.0.1'],
  };
  assert.equal(explicitStoreURL(echo, undefined), 'http://127.0.0.1:18427/store/');
  assert.equal(explicitStoreURL(echo, 'http://127.0.0.1:18427/store/game/tiny'), 'http://127.0.0.1:18427/store/game/tiny');
  assert.equal(explicitStoreURL(echo, 'http://store.steampowered.com/'), '');
  assert.equal(explicitStoreURL(steam, 'http://127.0.0.1:18427/store/'), '');
});

test('only ready plugins with a storefront open', () => {
  assert.ok(canOpenStore(steam));
  assert.ok(!canOpenStore(undefined));
  assert.ok(!canOpenStore({ ...steam, status: 'disabled' }));
  assert.ok(!canOpenStore({ ...steam, status: 'unapproved' }));
  assert.ok(!canOpenStore({ ...steam, storefront_url: '' }));
  assert.ok(!canOpenStore({ ...steam, storefront_url: undefined }));
});

test('top-level navigation stays inside the declared domains', () => {
  const domains = steam.storefront_domains!;
  assert.ok(navigationAllowed('https://store.steampowered.com/app/620/', domains));
  assert.ok(navigationAllowed('https://steamcommunity.com/login/', domains));
  // Third-party payment providers are NOT allow-listed: leaving for one is
  // blocked and the toolbar offers to open it in the browser instead.
  assert.ok(!navigationAllowed('https://checkout.stripe.com/pay', domains));
  assert.ok(!navigationAllowed('https://paypal.com/pay', domains));
  assert.ok(!navigationAllowed('http://127.0.0.1:18427/store/', domains));
  assert.ok(!navigationAllowed('steam://install/620', domains));
  assert.ok(!navigationAllowed('com.epicgames.launcher://open', domains));
  assert.ok(!navigationAllowed('file:///etc/passwd', domains));
  assert.ok(!navigationAllowed('javascript:alert(1)', domains));
  assert.ok(!navigationAllowed('nonsense', domains));
  assert.ok(!navigationAllowed('https://store.steampowered.com/', []));
});

test('blocked attempts name the outside host, silently refuse other schemes', () => {
  const domains = steam.storefront_domains!;
  assert.equal(blockedHost('https://checkout.stripe.com/pay', domains), 'checkout.stripe.com');
  assert.equal(blockedHost('https://store.steampowered.com/app/620/', domains), '');
  assert.equal(blockedHost('steam://install/620', domains), '');
  assert.equal(blockedHost('nonsense', domains), '');
});

test('popups open inside the domains only', () => {
  const domains = steam.storefront_domains!;
  assert.equal(popupURL('https://store.steampowered.com/login/', domains), 'https://store.steampowered.com/login/');
  assert.equal(popupURL('https://checkout.stripe.com/pay', domains), '');
  assert.equal(popupURL('steam://install/620', domains), '');
});

test('a stale close never hides the newly shown view', () => {
  assert.ok(shouldCloseView('echo', 'echo'));
  assert.ok(!shouldCloseView('echo-c', 'echo'));
  assert.ok(!shouldCloseView(null, 'echo'));
  assert.ok(!shouldCloseView('echo', ''));
  assert.ok(shouldCloseView('echo', undefined));
  assert.ok(!shouldCloseView(null, undefined));
});

test('the strip names the host and flags outside domains', () => {
  assert.deepEqual(stripState('https://store.steampowered.com/app/620/', steam.storefront_domains!), {
    host: 'store.steampowered.com', outside: false,
  });
  assert.deepEqual(stripState('https://checkout.stripe.com/pay', steam.storefront_domains!), {
    host: 'checkout.stripe.com', outside: true,
  });
  assert.deepEqual(stripState('nonsense', steam.storefront_domains!), { host: '', outside: false });
});

test('domain matching is exact or subdomain, loopback is loopback', () => {
  assert.ok(insideDomains('store.steampowered.com', ['steampowered.com']));
  assert.ok(insideDomains('steampowered.com', ['steampowered.com']));
  assert.ok(!insideDomains('evilfeepowered.com', ['steampowered.com']));
  assert.ok(!insideDomains('steampowered.com.evil.com', ['steampowered.com']));
  assert.ok(isLoopbackHost('127.0.0.1') && isLoopbackHost('127.0.0.2') && isLoopbackHost('localhost'));
  assert.ok(!isLoopbackHost('93.184.216.34') && !isLoopbackHost('example.com'));
});
