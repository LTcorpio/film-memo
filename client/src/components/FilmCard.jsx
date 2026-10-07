import { useState } from 'react';
import Icon from './Icon.jsx';
import PlatformTag from './PlatformTag.jsx';

const ICON_BASE = '/icon';

export default function FilmCard({ film, onClick, onContextMenu, ratingView = 'all' }) {
  const meta = film.metadata;
  const title = meta?.title || film.name;
  const year = film.releaseYear || meta?.releaseYear || '';
  const [imgLoaded, setImgLoaded] = useState(false);
  // 评分展示偏好：只渲染被允许的来源（none 时两个都是 false，整块徽章不渲染）
  const showDouban = ratingView === 'all' || ratingView === 'douban';
  const showImdb = ratingView === 'all' || ratingView === 'imdb';
  const hasDouban = showDouban && film.doubanRating > 0;
  const hasImdb = showImdb && film.imdbRating > 0;

  return (
    <div
      className="film-card"
      onClick={onClick}
      onContextMenu={onContextMenu}
    >
      <div className="poster">
        {meta?.posterUrl ? (
          <img
            src={meta.posterUrl}
            alt={title}
            loading="lazy"
            className={`poster-img${imgLoaded ? ' loaded' : ''}`}
            onLoad={() => setImgLoaded(true)}
          />
        ) : (
          <div className="poster-placeholder">
            <span className="ph-cat">{film.category}</span>
            <span className="ph-name">{film.name}</span>
          </div>
        )}
        {(!film.hasMetadata || film.watchStatus === 'watching') && (
          <div className="poster-badges">
            {/* 左上角徽标纵向排列，「正在观看」在上，避免与「无元数据」重叠 */}
            {film.watchStatus === 'watching' && (
              <span className="watching-badge" title="正在观看；看完后编辑记录补上结束观看日期">
                <Icon name="clock" size={11} /> 正在观看
              </span>
            )}
            {!film.hasMetadata && (
              <span className="no-meta-badge" title="未刮削元数据">
                <Icon name="alert" size={11} /> 无元数据
              </span>
            )}
          </div>
        )}
        {(hasDouban || hasImdb) && (
          <div className="rating-badges">
            {hasDouban && (
              <span className="rating-badge douban" title={`豆瓣评分 ${film.doubanRating}`}>
                <img className="rating-badge-logo" src={`${ICON_BASE}/douban.svg`} alt="" />
                {film.doubanRating.toFixed(1)}
              </span>
            )}
            {hasImdb && (
              <span className="rating-badge imdb" title={`IMDb 评分 ${film.imdbRating}`}>
                <img className="rating-badge-logo" src={`${ICON_BASE}/imdb.svg`} alt="" />
                {film.imdbRating.toFixed(1)}
              </span>
            )}
          </div>
        )}
      </div>
      <div className="card-body">
        <div className="card-line1">
          <span className="card-title" title={title}>{title}</span>
          {film.platforms.length > 0 && (
            <span className="card-plats">
              {/* 仅显示平台 LOGO（无外壳）；未匹配到 LOGO 的平台退化为名称文本 */}
              {film.platforms.map((p) => (
                <PlatformTag key={p} name={p} size={14} compact />
              ))}
            </span>
          )}
        </div>
        <div className="card-line2">
          <span className="cat-tag">{film.category}</span>
          {year && <span>{year}</span>}
          {film.totalEpisodes > 1 && <span className="ep-text">{film.totalEpisodes} 集</span>}
        </div>
        <div className="card-date-line">
          {film.startDate && (
            <span>
              {film.endDate && film.endDate !== film.startDate
                ? `${film.startDate} ~ ${film.endDate}`
                : film.startDate}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
