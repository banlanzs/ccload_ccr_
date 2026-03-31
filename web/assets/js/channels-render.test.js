const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function loadRenderSandbox(overrides = {}) {
  const source = fs.readFileSync(path.join(__dirname, 'channels-render.js'), 'utf8');
  const sandbox = {
    window: {
      t(key, params = {}) {
        if (key === 'channels.table.priority') return '优先级';
        if (key === 'channels.stats.healthScoreLabel') return '健康度';
        if (key === 'channels.stats.successRate') return `成功率 ${params.rate}`;
        if (key === 'channels.stats.firstByte') return '首字';
        if (key === 'channels.stats.calls') return '调用';
        if (key === 'stats.tooltipDuration') return '耗时';
        if (key === 'stats.unitTimes') return '次';
        if (key === 'common.success') return '成功';
        if (key === 'common.failed') return '失败';
        if (key === 'common.seconds') return '秒';
        return key;
      }
    },
    console
  };

  Object.assign(sandbox, overrides);

  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  return sandbox;
}

function loadRenderHelpers() {
  return loadRenderSandbox();
}

test('buildEffectivePriorityHtml 不渲染优先级和健康度标签', () => {
  const { buildEffectivePriorityHtml } = loadRenderHelpers();

  const html = buildEffectivePriorityHtml({
    priority: 110,
    effective_priority: 105,
    success_rate: 0.991
  });

  assert.ok(!html.includes('ch-priority-label'));
  assert.ok(html.includes('>110<'));
  assert.ok(html.includes('>105<'));
});

test('buildEffectivePriorityHtml 在健康度等于优先级时只显示一次优先级', () => {
  const { buildEffectivePriorityHtml } = loadRenderHelpers();

  const html = buildEffectivePriorityHtml({
    priority: 100,
    effective_priority: 100
  });

  assert.equal((html.match(/ch-priority-row/g) || []).length, 1);
  assert.equal((html.match(/>100</g) || []).length, 1);
  assert.ok(!html.includes('ch-priority-health'));
});

test('渠道卡片模板包含所有必需的操作按钮', () => {
  const channelsHtml = fs.readFileSync(path.join(__dirname, '..', '..', 'channels.html'), 'utf8');

  // 提取 tpl-channel-card 模板内容
  const templateMatch = channelsHtml.match(/<template id="tpl-channel-card">([\s\S]*?)<\/template>/);
  assert.ok(templateMatch, '未找到 tpl-channel-card 模板');

  const template = templateMatch[1];

  // 验证所有操作按钮都存在
  const requiredActions = ['edit', 'test', 'toggle', 'copy', 'delete'];

  requiredActions.forEach(action => {
    const actionPattern = new RegExp(`data-action="${action}"`);
    assert.match(template, actionPattern, `缺少 data-action="${action}" 按钮`);
  });

  // 验证按钮顺序：edit → test → toggle → copy → delete
  const actionOrder = [];
  const actionRegex = /data-action="(edit|test|toggle|copy|delete)"/g;
  let match;
  while ((match = actionRegex.exec(template)) !== null) {
    actionOrder.push(match[1]);
  }

  assert.deepEqual(actionOrder, ['edit', 'test', 'toggle', 'copy', 'delete'],
    '操作按钮顺序不正确');
});

test('复制按钮包含正确的国际化属性', () => {
  const channelsHtml = fs.readFileSync(path.join(__dirname, '..', '..', 'channels.html'), 'utf8');

  const templateMatch = channelsHtml.match(/<template id="tpl-channel-card">([\s\S]*?)<\/template>/);
  assert.ok(templateMatch, '未找到 tpl-channel-card 模板');

  const template = templateMatch[1];

  // 验证复制按钮的国际化属性
  assert.match(template, /data-action="copy"[\s\S]*?data-i18n="common\.copy"/,
    '复制按钮缺少 data-i18n="common.copy" 属性');
  assert.match(template, /data-action="copy"[\s\S]*?data-i18n-title="channels\.copyChannelTitle"/,
    '复制按钮缺少 data-i18n-title="channels.copyChannelTitle" 属性');
});

test('复制按钮包含必需的数据属性', () => {
  const channelsHtml = fs.readFileSync(path.join(__dirname, '..', '..', 'channels.html'), 'utf8');

  const templateMatch = channelsHtml.match(/<template id="tpl-channel-card">([\s\S]*?)<\/template>/);
  assert.ok(templateMatch, '未找到 tpl-channel-card 模板');

  const template = templateMatch[1];

  // 提取复制按钮的完整定义
  const copyButtonMatch = template.match(/<button[^>]*data-action="copy"[^>]*>/);
  assert.ok(copyButtonMatch, '未找到复制按钮');

  const copyButton = copyButtonMatch[0];

  // 验证必需的数据属性
  assert.match(copyButton, /data-channel-id="{{id}}"/, '复制按钮缺少 data-channel-id 属性');
  assert.match(copyButton, /data-channel-name="{{name}}"/, '复制按钮缺少 data-channel-name 属性');

  // 验证样式类
  assert.match(copyButton, /class="[^"]*btn-icon[^"]*"/, '复制按钮缺少 btn-icon 类');
  assert.match(copyButton, /class="[^"]*channel-action-btn[^"]*"/, '复制按钮缺少 channel-action-btn 类');
});

test('buildChannelTimingHtml 渲染耗时和带单位的调用汇总', () => {
  const { buildChannelTimingHtml } = loadRenderHelpers();

  const html = buildChannelTimingHtml({
    avgFirstByteTimeSeconds: 2.3,
    avgDurationSeconds: 22.23,
    success: 17,
    error: 3
  });

  assert.match(html, /首字/);
  assert.match(html, /耗时/);
  assert.match(html, /调用/);
  assert.match(html, />2\.30秒</);
  assert.match(html, />22\.23秒</);
  assert.match(html, /17<\/span>\/<span style="color: var\(--error-600\);">3<\/span>次/);
  assert.doesNotMatch(html, />成功</);
  assert.doesNotMatch(html, />失败</);
});

test('initChannelEventDelegation 允许表头全选 checkbox 触发可见渠道批量选择', () => {
  const listeners = {};
  const container = {
    dataset: {},
    addEventListener(type, handler) {
      listeners[type] = handler;
    }
  };
  let toggleCalls = 0;

  const { initChannelEventDelegation } = loadRenderSandbox({
    document: {
      getElementById(id) {
        return id === 'channels-container' ? container : null;
      }
    },
    toggleVisibleChannelsSelection() {
      toggleCalls += 1;
    },
    selectedChannelIds: new Set(),
    normalizeSelectedChannelID(value) {
      return value;
    }
  });

  initChannelEventDelegation();

  const headerCheckbox = {
    id: 'visibleSelectionCheckbox',
    closest(selector) {
      if (selector === '#visibleSelectionCheckbox') return this;
      return null;
    }
  };

  listeners.change({ target: headerCheckbox });

  assert.equal(toggleCalls, 1);
});
