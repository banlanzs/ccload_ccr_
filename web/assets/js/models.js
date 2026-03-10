// 模型管理页面
const t = window.t;

let virtualModels = [];
let channels = [];
let currentVirtualModelId = null;
let currentAssociations = [];
let lastPreviewData = null;

function esc(value) {
  if (typeof window.escapeHtml === 'function') {
    return window.escapeHtml(value == null ? '' : String(value));
  }
  const div = document.createElement('div');
  div.textContent = value == null ? '' : String(value);
  return div.innerHTML;
}

function formatDateTime(isoString) {
  if (!isoString) return '-';
  const locale = window.i18n?.getLocale?.() || 'zh-CN';
  const dt = new Date(isoString);
  if (Number.isNaN(dt.getTime())) return '-';
  return dt.toLocaleString(locale);
}

function openModal(id) {
  const modal = document.getElementById(id);
  if (!modal) return;
  modal.style.display = 'flex';
}

function closeModal(id) {
  const modal = document.getElementById(id);
  if (!modal) return;
  modal.style.display = 'none';
}

function closeAllModals() {
  document.querySelectorAll('.modal').forEach((modal) => {
    modal.style.display = 'none';
  });
}

async function loadVirtualModels() {
  try {
    const data = await fetchDataWithAuth('/admin/virtual-models');
    virtualModels = Array.isArray(data) ? data : [];
    renderVirtualModels();
  } catch (err) {
    console.error('Failed to load virtual models', err);
    window.showError?.(t('models.loadFailed'));
  }
}

async function loadChannels() {
  try {
    const data = await fetchDataWithAuth('/admin/channels');
    channels = Array.isArray(data) ? data : [];
  } catch (err) {
    console.error('Failed to load channels', err);
    channels = [];
  }
}

async function loadAssociations(virtualModelId) {
  try {
    const data = await fetchDataWithAuth(`/admin/model-associations?virtual_model_id=${virtualModelId}`);
    currentAssociations = Array.isArray(data) ? data : [];
    renderAssociations();
  } catch (err) {
    console.error('Failed to load associations', err);
    window.showError?.(t('models.loadAssociationsFailed'));
  }
}

function getMatchTypeText(type) {
  switch (String(type || '')) {
    case 'exact':
      return t('models.matchTypeExact');
    case 'prefix':
      return t('models.matchTypePrefix');
    case 'suffix':
      return t('models.matchTypeSuffix');
    case 'contains':
      return t('models.matchTypeContains');
    case 'regex':
      return t('models.matchTypeRegex');
    case 'wildcard':
      return t('models.matchTypeWildcard');
    default:
      return String(type || '-');
  }
}

function renderVirtualModels() {
  const container = document.getElementById('virtualModelsContainer');
  if (!container) return;

  if (virtualModels.length === 0) {
    container.innerHTML = `
      <div class="empty-state">
        <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
          <line x1="9" y1="9" x2="15" y2="9"></line>
          <line x1="9" y1="13" x2="15" y2="13"></line>
          <line x1="9" y1="17" x2="13" y2="17"></line>
        </svg>
        <h3>${esc(t('models.noVirtualModels'))}</h3>
        <p>${esc(t('models.createFirstHint'))}</p>
      </div>
    `;
    return;
  }

  container.innerHTML = virtualModels.map((vm) => {
    const id = Number(vm.id) || 0;
    const enabled = !!vm.enabled;
    const count = Number(vm.associations_count || 0);
    return `
      <div class="model-item" data-id="${id}">
        <div class="model-header">
          <div class="model-info">
            <h3 class="model-name">${esc(vm.name || '-')}</h3>
            <span class="model-status ${enabled ? 'enabled' : 'disabled'}">
              ${esc(enabled ? t('common.enabled') : t('common.disabled'))}
            </span>
          </div>
          <div class="model-actions">
            <button class="btn btn-sm btn-primary" type="button" data-action="manage-associations" data-id="${id}">
              ${esc(t('models.manageAssociations'))}
            </button>
            <button class="btn btn-sm btn-secondary" type="button" data-action="edit-model" data-id="${id}">
              ${esc(t('common.edit'))}
            </button>
            <button class="btn btn-sm btn-danger" type="button" data-action="delete-model" data-id="${id}">
              ${esc(t('common.delete'))}
            </button>
          </div>
        </div>
        <div class="model-description">${esc(vm.description || t('models.noDescription'))}</div>
        <div class="model-meta">
          <span>${esc(t('models.associationsCount', { count }))}</span>
          <span class="model-date">${esc(formatDateTime(vm.created_at))}</span>
        </div>
      </div>
    `;
  }).join('');
}

function renderAssociations() {
  const container = document.getElementById('associationsContainer');
  if (!container) return;

  if (!currentAssociations.length) {
    container.innerHTML = `<p class="no-rules">${esc(t('models.noAssociations'))}</p>`;
    return;
  }

  container.innerHTML = currentAssociations.map((assoc) => {
    const id = Number(assoc.id) || 0;
    const enabled = !!assoc.enabled;
    const channelId = Number(assoc.channel_id || 0);
    const channelTags = assoc.channel_tags || '';
    let channelInfo = '';

    if (channelId > 0) {
      channelInfo = `${esc(assoc.channel_name || `#${channelId}`)}`;
    } else if (channelTags.trim()) {
      channelInfo = `${esc(t('models.tags'))}: ${esc(channelTags)}`;
    } else {
      channelInfo = esc(t('models.scopeGlobal'));
    }

    return `
      <div class="association-item" data-id="${id}">
        <div class="association-header">
          <div class="association-info">
            <span class="association-channel">${channelInfo}</span>
            <span class="association-match-type">${esc(getMatchTypeText(assoc.match_type))}</span>
            <span class="association-priority">P${esc(assoc.priority ?? 0)}</span>
            <span class="association-status ${enabled ? 'enabled' : 'disabled'}">
              ${esc(enabled ? t('common.enabled') : t('common.disabled'))}
            </span>
          </div>
          <div class="association-actions">
            <button class="btn btn-sm btn-secondary" type="button" data-action="edit-association" data-id="${id}">
              ${esc(t('common.edit'))}
            </button>
            <button class="btn btn-sm btn-danger" type="button" data-action="delete-association" data-id="${id}">
              ${esc(t('common.delete'))}
            </button>
          </div>
        </div>
        <div class="association-pattern">
          <span>${esc(t('models.pattern'))}:</span>
          <code>${esc(assoc.pattern || '')}</code>
        </div>
      </div>
    `;
  }).join('');
}

function renderPreviewResult(data) {
  const resultContainer = document.getElementById('previewResult');
  if (!resultContainer) return;

  const matchedRules = Array.isArray(data?.matched_rules) ? data.matched_rules : [];
  const candidates = Array.isArray(data?.candidates) ? data.candidates : [];

  const matchedHtml = matchedRules.length
    ? `<ul class="matched-rules-list">${matchedRules.map((rule) => `
        <li>
          <strong>${esc(rule.channel_name || `#${rule.channel_id || '-'}`)}</strong>
          <span class="badge">${esc(getMatchTypeText(rule.match_type))}</span>
          <code>${esc(rule.pattern || '')}</code>
          <span class="priority">P${esc(rule.priority ?? 0)}</span>
        </li>
      `).join('')}</ul>`
    : `<p class="no-rules">${esc(t('models.noMatchedRules'))}</p>`;

  const candidatesHtml = candidates.length
    ? `<ul class="candidates-list">${candidates.map((c) => `
        <li>
          <strong>${esc(c.channel_name || `#${c.channel_id || '-'}`)}</strong>
          <span>${esc(t('models.virtualModel'))}: ${esc(c.virtual_model || '-')}</span>
          <span>${esc(t('models.resolvedModel'))}: ${esc(c.resolved_model || '-')}</span>
        </li>
      `).join('')}</ul>`
    : `<p class="no-channels">${esc(t('models.noCandidates'))}</p>`;

  resultContainer.innerHTML = `
    <div class="preview-results">
      <h4>${esc(t('models.matchedRules'))}</h4>
      ${matchedHtml}
      <h4>${esc(t('models.candidateChannels'))}</h4>
      ${candidatesHtml}
      ${data?.message ? `<div class="preview-message">${esc(data.message)}</div>` : ''}
    </div>
  `;
}

function updateScopeFields() {
  const scope = document.getElementById('associationScope')?.value || 'channel';
  const channelGroup = document.getElementById('channelSelectGroup');
  const tagsGroup = document.getElementById('channelTagsGroup');
  const channelSelect = document.getElementById('associationChannelId');

  if (!channelGroup || !tagsGroup || !channelSelect) return;

  if (scope === 'channel') {
    channelGroup.style.display = 'block';
    tagsGroup.style.display = 'none';
    channelSelect.required = true;
  } else if (scope === 'tags') {
    channelGroup.style.display = 'none';
    tagsGroup.style.display = 'block';
    channelSelect.required = false;
  } else {
    channelGroup.style.display = 'none';
    tagsGroup.style.display = 'none';
    channelSelect.required = false;
  }
}

function populateChannelOptions(selectedChannelId) {
  const select = document.getElementById('associationChannelId');
  if (!select) return;

  const selected = String(selectedChannelId || '');
  const baseOption = `<option value="">${esc(t('models.selectChannel'))}</option>`;

  const options = channels.map((ch) => {
    const id = Number(ch.id) || 0;
    const name = ch.name || `#${id}`;
    const type = ch.type || '-';
    const isSelected = String(id) === selected ? ' selected' : '';
    return `<option value="${id}"${isSelected}>${esc(`${name} (${type})`)}</option>`;
  }).join('');

  select.innerHTML = baseOption + options;
}

function openVirtualModelModal(id) {
  const title = document.getElementById('virtualModelModalTitle');
  const idInput = document.getElementById('virtualModelId');
  const nameInput = document.getElementById('virtualModelName');
  const descInput = document.getElementById('virtualModelDescription');
  const enabledInput = document.getElementById('virtualModelEnabled');

  if (!title || !idInput || !nameInput || !descInput || !enabledInput) return;

  idInput.value = '';
  nameInput.value = '';
  descInput.value = '';
  enabledInput.checked = true;

  if (id) {
    const vm = virtualModels.find((m) => Number(m.id) === Number(id));
    if (vm) {
      title.textContent = t('models.editVirtualModel');
      idInput.value = String(vm.id || '');
      nameInput.value = vm.name || '';
      descInput.value = vm.description || '';
      enabledInput.checked = !!vm.enabled;
    }
  } else {
    title.textContent = t('models.createVirtualModel');
  }

  openModal('virtualModelModal');
}

async function saveVirtualModel() {
  const id = document.getElementById('virtualModelId')?.value?.trim() || '';
  const name = document.getElementById('virtualModelName')?.value?.trim() || '';
  const description = document.getElementById('virtualModelDescription')?.value?.trim() || '';
  const enabled = !!document.getElementById('virtualModelEnabled')?.checked;

  if (!name) {
    window.showError?.(t('models.nameRequired'));
    return;
  }

  const payload = { name, description, enabled };

  try {
    if (id) {
      await fetchDataWithAuth(`/admin/virtual-models/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
    } else {
      await fetchDataWithAuth('/admin/virtual-models', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
    }

    closeModal('virtualModelModal');
    await loadVirtualModels();
    window.showSuccess?.(t('models.saveSuccess'));
  } catch (err) {
    console.error('Failed to save virtual model', err);
    window.showError?.(t('models.saveFailed'));
  }
}

async function deleteVirtualModel(id) {
  if (!id) return;
  if (!confirm(t('models.deleteConfirm'))) return;

  try {
    await fetchDataWithAuth(`/admin/virtual-models/${id}`, { method: 'DELETE' });
    await loadVirtualModels();
    window.showSuccess?.(t('models.deleteSuccess'));
  } catch (err) {
    console.error('Failed to delete virtual model', err);
    window.showError?.(t('models.deleteFailed'));
  }
}

async function openAssociationsModal(virtualModelId) {
  currentVirtualModelId = Number(virtualModelId) || 0;

  const title = document.getElementById('associationsModalTitle');
  if (title) {
    const vm = virtualModels.find((m) => Number(m.id) === currentVirtualModelId);
    title.textContent = vm ? `${t('models.manageAssociations')}: ${vm.name}` : t('models.manageAssociations');
  }

  await loadAssociations(currentVirtualModelId);
  openModal('associationsModal');
}

function openAssociationEditModal(id) {
  const title = document.getElementById('associationEditModalTitle');
  const idInput = document.getElementById('associationId');
  const scopeInput = document.getElementById('associationScope');
  const channelIdInput = document.getElementById('associationChannelId');
  const channelTagsInput = document.getElementById('associationChannelTags');
  const typeInput = document.getElementById('associationMatchType');
  const patternInput = document.getElementById('associationPattern');
  const priorityInput = document.getElementById('associationPriority');
  const enabledInput = document.getElementById('associationEnabled');
  const excludeChannelIdsInput = document.getElementById('excludeChannelIds');
  const excludeChannelTagsInput = document.getElementById('excludeChannelTags');
  const excludeChannelNamePatternInput = document.getElementById('excludeChannelNamePattern');

  if (!title || !idInput || !scopeInput || !channelIdInput || !channelTagsInput ||
      !typeInput || !patternInput || !priorityInput || !enabledInput) return;

  idInput.value = '';
  scopeInput.value = 'channel';
  channelIdInput.value = '';
  channelTagsInput.value = '';
  typeInput.value = 'exact';
  patternInput.value = '';
  priorityInput.value = '100';
  enabledInput.checked = true;

  // 清空排除字段
  if (excludeChannelIdsInput) excludeChannelIdsInput.value = '';
  if (excludeChannelTagsInput) excludeChannelTagsInput.value = '';
  if (excludeChannelNamePatternInput) excludeChannelNamePatternInput.value = '';

  if (id) {
    const assoc = currentAssociations.find((a) => Number(a.id) === Number(id));
    if (assoc) {
      title.textContent = t('models.editAssociation');
      idInput.value = String(assoc.id || '');

      // Determine scope from channel_id and channel_tags
      const channelId = Number(assoc.channel_id || 0);
      const channelTags = assoc.channel_tags || '';

      if (channelId > 0) {
        scopeInput.value = 'channel';
        populateChannelOptions(channelId);
      } else if (channelTags.trim()) {
        scopeInput.value = 'tags';
        channelTagsInput.value = channelTags;
      } else {
        scopeInput.value = 'global';
      }

      updateScopeFields();
      typeInput.value = assoc.match_type || 'exact';
      patternInput.value = assoc.pattern || '';
      priorityInput.value = String(assoc.priority ?? 100);
      enabledInput.checked = !!assoc.enabled;

      // 加载排除字段
      if (excludeChannelIdsInput) excludeChannelIdsInput.value = assoc.exclude_channel_ids || '';
      if (excludeChannelTagsInput) excludeChannelTagsInput.value = assoc.exclude_channel_tags || '';
      if (excludeChannelNamePatternInput) excludeChannelNamePatternInput.value = assoc.exclude_channel_name_pattern || '';
    }
  } else {
    title.textContent = t('models.addAssociation');
    populateChannelOptions('');
    updateScopeFields();
  }

  openModal('associationEditModal');
}

async function saveAssociation() {
  const id = document.getElementById('associationId')?.value?.trim() || '';
  const scope = document.getElementById('associationScope')?.value || 'channel';
  const matchType = document.getElementById('associationMatchType')?.value || 'exact';
  const pattern = document.getElementById('associationPattern')?.value?.trim() || '';
  const priority = Number(document.getElementById('associationPriority')?.value || 0);
  const enabled = !!document.getElementById('associationEnabled')?.checked;

  // 获取排除字段
  const excludeChannelIds = document.getElementById('excludeChannelIds')?.value?.trim() || '';
  const excludeChannelTags = document.getElementById('excludeChannelTags')?.value?.trim() || '';
  const excludeChannelNamePattern = document.getElementById('excludeChannelNamePattern')?.value?.trim() || '';

  let channelId = 0;
  let channelTags = '';

  if (scope === 'channel') {
    channelId = Number(document.getElementById('associationChannelId')?.value || 0);
    if (!channelId) {
      window.showError?.(t('models.channelRequired'));
      return;
    }
  } else if (scope === 'tags') {
    channelTags = document.getElementById('associationChannelTags')?.value?.trim() || '';
    if (!channelTags) {
      window.showError?.(t('models.channelTagsRequired'));
      return;
    }
  }

  if (!pattern) {
    window.showError?.(t('models.patternRequired'));
    return;
  }

  const payload = {
    virtual_model_id: currentVirtualModelId,
    channel_id: channelId,
    channel_tags: channelTags,
    match_type: matchType,
    pattern,
    priority,
    enabled,
    exclude_channel_ids: excludeChannelIds,
    exclude_channel_tags: excludeChannelTags,
    exclude_channel_name_pattern: excludeChannelNamePattern
  };

  try {
    if (id) {
      await fetchDataWithAuth(`/admin/model-associations/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
    } else {
      await fetchDataWithAuth('/admin/model-associations', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
    }

    closeModal('associationEditModal');
    await loadAssociations(currentVirtualModelId);
    window.showSuccess?.(t('models.associationSaveSuccess'));
  } catch (err) {
    console.error('Failed to save association', err);
    window.showError?.(t('models.associationSaveFailed'));
  }
}

async function deleteAssociation(id) {
  if (!id) return;
  if (!confirm(t('models.deleteAssociationConfirm'))) return;

  try {
    await fetchDataWithAuth(`/admin/model-associations/${id}`, { method: 'DELETE' });
    await loadAssociations(currentVirtualModelId);
    window.showSuccess?.(t('models.associationDeleteSuccess'));
  } catch (err) {
    console.error('Failed to delete association', err);
    window.showError?.(t('models.associationDeleteFailed'));
  }
}

function openPreviewModal() {
  lastPreviewData = null;
  const result = document.getElementById('previewResult');
  if (result) result.innerHTML = '';
  openModal('previewModal');
}

async function runRoutingPreview() {
  const model = document.getElementById('previewModel')?.value?.trim() || '';
  const requestType = document.getElementById('previewRequestType')?.value || '';

  if (!model) {
    window.showError?.(t('models.modelRequired'));
    return;
  }

  try {
    const data = await fetchDataWithAuth('/admin/model-associations/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ model, request_type: requestType })
    });

    lastPreviewData = data || null;
    renderPreviewResult(data || {});
  } catch (err) {
    console.error('Failed to run routing preview', err);
    window.showError?.(t('models.previewFailed'));
  }
}

function bindEvents() {
  document.getElementById('createVirtualModelBtn')?.addEventListener('click', () => openVirtualModelModal());
  document.getElementById('saveVirtualModelBtn')?.addEventListener('click', saveVirtualModel);
  document.getElementById('addAssociationBtn')?.addEventListener('click', () => openAssociationEditModal());
  document.getElementById('saveAssociationBtn')?.addEventListener('click', saveAssociation);
  document.getElementById('previewRoutingBtn')?.addEventListener('click', openPreviewModal);
  document.getElementById('runPreviewBtn')?.addEventListener('click', runRoutingPreview);

  document.querySelectorAll('[data-close-modal]').forEach((btn) => {
    btn.addEventListener('click', () => {
      const target = btn.getAttribute('data-close-modal');
      if (target) closeModal(target);
    });
  });

  document.querySelectorAll('.modal').forEach((modal) => {
    modal.addEventListener('click', (event) => {
      if (event.target === modal) {
        modal.style.display = 'none';
      }
    });
  });

  const vmContainer = document.getElementById('virtualModelsContainer');
  vmContainer?.addEventListener('click', (event) => {
    const btn = event.target.closest('button[data-action]');
    if (!btn) return;

    const action = btn.dataset.action;
    const id = Number(btn.dataset.id || 0);

    if (action === 'manage-associations') {
      openAssociationsModal(id);
      return;
    }
    if (action === 'edit-model') {
      openVirtualModelModal(id);
      return;
    }
    if (action === 'delete-model') {
      deleteVirtualModel(id);
    }
  });

  const associationsContainer = document.getElementById('associationsContainer');
  associationsContainer?.addEventListener('click', (event) => {
    const btn = event.target.closest('button[data-action]');
    if (!btn) return;

    const action = btn.dataset.action;
    const id = Number(btn.dataset.id || 0);

    if (action === 'edit-association') {
      openAssociationEditModal(id);
      return;
    }
    if (action === 'delete-association') {
      deleteAssociation(id);
    }
  });
}

function onLocaleChanged() {
  window.i18n?.translatePage?.();
  renderVirtualModels();
  renderAssociations();
  if (lastPreviewData) {
    renderPreviewResult(lastPreviewData);
  }

  if (currentVirtualModelId) {
    const title = document.getElementById('associationsModalTitle');
    const vm = virtualModels.find((m) => Number(m.id) === currentVirtualModelId);
    if (title) {
      title.textContent = vm ? `${t('models.manageAssociations')}: ${vm.name}` : t('models.manageAssociations');
    }
  }
}

async function initModelsPage() {
  initTopbar('models');
  window.i18n?.translatePage?.();
  bindEvents();

  await Promise.all([loadVirtualModels(), loadChannels()]);

  window.i18n?.onLocaleChange?.(() => {
    onLocaleChanged();
  });
}

document.addEventListener('DOMContentLoaded', initModelsPage);
