# One CLI · Brand Assets v1

> 双缺口 o 方向 / torch amber 主色 / 生成于品牌敲定阶段。

## 文件清单

| 文件 | 用途 | 尺寸 |
|---|---|---|
| `icon.svg` | 图标主版（torch amber） | 120×120 viewBox |
| `icon-mono.svg` | 图标单色（黑） | 120×120 |
| `icon-inverted.svg` | 图标暗底（白） | 120×120 |
| `logo.svg` | 横排 wordmark 主版 | 500×120 |
| `logo-mono.svg` | 横排 wordmark 单色 | 500×120 |
| `logo-inverted.svg` | 横排 wordmark 暗底 | 500×120 |
| `logo-stacked.svg` | 竖排堆叠版（app icon / 头像） | 200×240 |
| `favicon.svg` | 浏览器 tab / 书签 | 32×32 |

## 颜色规范

| 用途 | 色值 | 备注 |
|---|---|---|
| Torch（主色） | `#ea580c` | Tailwind orange-600，呼应 torchstellar 母品牌 |
| Ink（文字/单色图标） | `#0a0a0a` | 接近纯黑但留一点温度 |
| Paper（暗底文字/图标） | `#fafafa` | 接近纯白但不刺眼 |
| Muted（次级文字、tagline） | `#737373` | Tailwind neutral-500 |

## 几何规范

```
图标外径    R
描边宽度    R × 0.45
缺口角度    50°（每个）
缺口位置    225° 和 45°（math angle，绕中心 180° 旋转对称）
线帽        round
```

按此比例缩放，任何尺寸都能保持视觉一致。当 R < 8（约 16-20px 渲染）时双缺口会被圆头吃掉，需要切到 butt cap 或换更宽缺口角度。`favicon.svg` 已经按 R=12 / 描边 5 优化过，再小请单独处理。

## 用法

### Web

```html
<!-- 横排，自适应宽度 -->
<img src="/brand/logo.svg" alt="One CLI" style="height: 32px" />

<!-- 暗底场景 -->
<img src="/brand/logo-inverted.svg" alt="One CLI" />

<!-- favicon -->
<link rel="icon" type="image/svg+xml" href="/brand/favicon.svg" />
```

### Docs 站（Next.js + Fumadocs）

放进 `apps/docs/public/brand/`，然后在 `app/layout.tsx` 引用 favicon，在 `app/page.tsx` 的 hero 引 logo.svg。

### README badges

GitHub README hero 推荐用 `logo-inverted.svg`——GitHub 暗色主题下 torch 琥珀虽然能看见但偏暗，纯白单色更稳。

### 终端 banner

CLI 启动时如果要打印 ASCII banner，参考几何规范用 box-drawing 字符自己拼，或贴一段 `icon.svg` 的 base64 让用 sixel/iTerm2 inline image 协议的终端渲染。

## 字体说明

**当前 SVG 里的 wordmark 文字用的是 `Inter` fallback `Helvetica Neue` `Arial`**。这是临时方案——为了让 SVG 在没有特殊 webfont 的地方也能渲染。

**production 用法建议**：

1. 选定字体（推荐 **Geist Sans** [SIL OFL]，备选 Inter / Söhne / IBM Plex Sans）
2. 在 Figma / Illustrator / Inkscape 里打开 wordmark SVG
3. 把 `<text>` 转成 outline / path（Figma: 右键 → Outline Stroke 不是这个，是 Flatten Selection；Illustrator: Type → Create Outlines）
4. 重新导出 SVG —— 文字变成 `<path>`，跨环境渲染完全一致

转 outline 之后字体可以不再依赖系统，logo SVG 真正"独立"。

## 留白规则

logo 周围预留至少 1× 图标外径 的留白。横排 wordmark 周围至少 0.5× 图标外径。

不要：
- 把 logo 放在密集纹理或杂乱图片上
- 拉伸或压缩（保持 viewBox 比例）
- 改色，除非用本文档列出的色值

## 下一步可能要补的

- `logo-tagline.svg`（hero 用，tagline 待你定稿）
- `og-card.svg`（1200×630 社交分享卡，需要选好 tagline 和 hero 配色）
- 字体最终确定后，所有 wordmark SVG 转 outline 重出
- 16×16 favicon 降级版（双缺口在该尺寸读不出来时）

---

生成时间：2026-05-11
版本：v1（torch + 双缺口 + Inter fallback）
