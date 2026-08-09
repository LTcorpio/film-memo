/**
 * 观看平台 → LOGO（来自 client/public/icon/*.svg）+ 平台名称。
 * 匹配规则：平台字符串小写后，按 aliases 关键词包含匹配（先到先得）。
 * 例："夸克网盘(离线)" 命中 alias "夸克" → 使用「夸克网盘.svg」。
 * 显示文本保留原始字符串（如 "夸克网盘(离线)"），图标用匹配到的 SVG。
 */

// 平台配置：{ file, name, aliases }
// file: client/public/icon/ 下的文件名（不含路径）
// name: 用于 tooltip / 未匹配时的官方名参考
// aliases: 归一化（小写、去空白）后包含即命中；注意短词放后面避免误伤
const PLATFORMS = [
  { file: '爱奇艺.svg', name: '爱奇艺', aliases: ['iqiyi', '爱奇艺'] },
  { file: '腾讯视频.svg', name: '腾讯视频', aliases: ['tencent', '腾讯视频', '腾讯', 'qq视频'] },
  { file: '优酷.svg', name: '优酷', aliases: ['youku', '优酷'] },
  { file: '西瓜视频.svg', name: '西瓜视频', aliases: ['xigua', '西瓜视频', '西瓜'] },
  { file: '阿里云盘.svg', name: '阿里云盘', aliases: ['aliyun', '阿里云盘', 'alipan', '阿里'] },
  { file: '夸克网盘.svg', name: '夸克网盘', aliases: ['quark', '夸克网盘', '夸克'] },
  { file: '百度网盘.svg', name: '百度网盘', aliases: ['baidu', '百度网盘', '百度'] },
  { file: 'UC网盘.svg', name: 'UC网盘', aliases: ['uc网盘', 'uc盘'] },
  { file: 'bilibili.svg', name: '哔哩哔哩', aliases: ['bilibili', '哔哩哔哩', '哔哩', 'b站'] },
  { file: 'youtube.svg', name: 'YouTube', aliases: ['youtube', '油管'] },
  { file: '央视.svg', name: '央视', aliases: ['cctv', '央视'] },
  { file: '迅雷.svg', name: '迅雷', aliases: ['thunder', 'xunlei', '迅雷'] },
  { file: '韩剧TV.svg', name: '韩剧TV', aliases: ['韩剧tv', '韩剧'] },
];

const ICON_BASE = '/icon';

function normalize(s) {
  return String(s || '').trim().toLowerCase();
}

/** 按平台字符串匹配配置；未匹配返回 null（调用方可用原文本兜底） */
export function matchPlatform(raw) {
  const key = normalize(raw);
  if (!key) return null;
  for (const p of PLATFORMS) {
    for (const a of p.aliases) {
      if (key.includes(a)) return p;
    }
  }
  return null;
}

export const ALL_PLATFORMS = PLATFORMS;

/**
 * 渲染单个平台标签：SVG LOGO + 名称。
 * size: 图标像素，默认 16。compact: 是否仅显示图标（无名称）。
 * 未匹配到 SVG 时，回退为普通 chip 显示原文本。
 */
export default function PlatformTag({ name, size = 16, compact = false, className = '' }) {
  const matched = matchPlatform(name);
  if (!matched) {
    // 未匹配到 SVG 时，复用 platform-tag 样式但不渲染图标；
    // compact 模式下也无图标可显示，故始终显示名称，否则标签为空不可见。
    return (
      <span className={`platform-tag platform-tag-no-logo ${className}`} title={name}>
        <span className="platform-name">{name}</span>
      </span>
    );
  }
  // URL 编码文件名，避免中文 / 特殊字符在 URL 中出问题
  const src = `${ICON_BASE}/${encodeURIComponent(matched.file)}`;
  return (
    <span className={`platform-tag ${className}`} title={matched.name}>
      <img
        src={src}
        alt={matched.name}
        width={size}
        height={size}
        className="platform-logo"
        loading="lazy"
      />
      {!compact && <span className="platform-name">{name}</span>}
    </span>
  );
}
