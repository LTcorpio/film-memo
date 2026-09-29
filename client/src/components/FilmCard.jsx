import { useState } from 'react';
import Icon from './Icon.jsx';
import PlatformTag from './PlatformTag.jsx';

const ICON_BASE = '/icon';

export default function FilmCard({ film, onClick, onContextMenu }) {
  const meta = film.metadata;
  const title = meta?.title || film.name;
  const year = film.releaseYear || meta?.releaseYear || '';
  const [imgLoaded, setImgLoaded] = useState(false);

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
        {!film.hasMetadata && (
          <span className="no-meta-badge" title="未刮削元数据">
            <Icon name="alert" size={11} /> 无元数据
          </span>
        )}
        {(film.doubanRating > 0 || film.imdbRating > 0) && (
          <div className="rating-badges">
            {film.doubanRating > 0 && (
              <span className="rating-badge douban" title={`豆瓣评分 ${film.doubanRating}`}>
                <img className="rating-badge-logo" src={`${ICON_BASE}/douban.svg`} alt="" />
                {film.doubanRating.toFixed(1)}
              </span>
            )}
            {film.imdbRating > 0 && (
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
