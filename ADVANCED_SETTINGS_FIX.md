# 🔧 高级设置自动展开功能修复

## 问题描述

**原始问题**：
- 用户编辑渠道并启用 CCR 或格式转换后保存
- 再次打开该渠道编辑时，高级设置区域没有自动展开
- 用户需要手动点击"高级设置"才能看到已启用的配置

## 修复方案

### 1. HTML 修改
给 `<details>` 元素添加 ID，便于 JavaScript 控制：

```html
<!-- 修改前 -->
<details>
  <summary>高级设置</summary>
  ...
</details>

<!-- 修改后 -->
<details id="advancedSettingsDetails">
  <summary>高级设置</summary>
  ...
</details>
```

**文件**：`ccLoad/web/channels.html` (第 349 行)

### 2. JavaScript 修改

#### 2.1 editChannel() 函数
在加载渠道配置后，检查是否启用了高级功能，自动展开：

```javascript
// 如果启用了 CCR 或格式转换，自动展开高级设置
const hasAdvancedConfig = (channel.enable_ccr || channel.enable_conversion);
const advancedDetails = document.getElementById('advancedSettingsDetails');
if (advancedDetails) {
  advancedDetails.open = hasAdvancedConfig;
}
```

**位置**：`ccLoad/web/assets/js/channels-modals.js` (第 86-91 行)

#### 2.2 copyChannel() 函数
复制渠道时也应用相同逻辑：

```javascript
// 如果启用了 CCR 或格式转换，自动展开高级设置
const hasAdvancedConfig = (channel.enable_ccr || channel.enable_conversion);
const advancedDetails = document.getElementById('advancedSettingsDetails');
if (advancedDetails) {
  advancedDetails.open = hasAdvancedConfig;
}
```

**位置**：`ccLoad/web/assets/js/channels-modals.js` (第 310-315 行)

## 修复后的行为

### 场景 1：编辑启用了高级功能的渠道
1. 用户点击"编辑"按钮
2. 弹窗打开
3. **高级设置自动展开** ✅
4. 显示已启用的 CCR 或格式转换配置

### 场景 2：编辑未启用高级功能的渠道
1. 用户点击"编辑"按钮
2. 弹窗打开
3. **高级设置保持折叠** ✅
4. 用户可以手动点击展开

### 场景 3：复制渠道
1. 用户点击"复制"按钮
2. 弹窗打开
3. **如果原渠道有高级配置，自动展开** ✅
4. 用户可以看到复制的配置

### 场景 4：新增渠道
1. 用户点击"添加渠道"按钮
2. 弹窗打开
3. **高级设置默认折叠** ✅
4. 用户可以手动展开配置

## 技术细节

### HTML5 `<details>` 元素
- `open` 属性控制展开/折叠状态
- `details.open = true` → 展开
- `details.open = false` → 折叠

### 判断逻辑
```javascript
const hasAdvancedConfig = (channel.enable_ccr || channel.enable_conversion);
```

只要满足以下任一条件，就自动展开：
- `enable_ccr === true` (启用了 Legacy CCR)
- `enable_conversion === true` (启用了新格式转换)

## 测试验证

### 测试步骤
1. 清除浏览器缓存（`Ctrl + Shift + R`）
2. 访问 `http://localhost:8083/web/channels.html`
3. 编辑一个启用了 CCR 或格式转换的渠道
4. 验证高级设置自动展开
5. 编辑一个未启用高级功能的渠道
6. 验证高级设置保持折叠

### 预期结果
- ✅ 启用了高级功能的渠道：自动展开
- ✅ 未启用高级功能的渠道：保持折叠
- ✅ 复制渠道：继承原渠道的展开状态
- ✅ 新增渠道：默认折叠

## 部署状态

- ✅ HTML 修改完成
- ✅ JavaScript 修改完成
- ✅ 服务重新构建
- ✅ 服务重启成功
- ✅ 健康检查通过

**服务地址**：http://localhost:8083
**版本**：v1.73.7-1-g5362750

## 相关文件

- `ccLoad/web/channels.html` (第 349 行)
- `ccLoad/web/assets/js/channels-modals.js` (第 86-91, 310-315 行)

---

**修复完成时间**：2026-02-17 12:59
**状态**：✅ 已部署，可以测试
