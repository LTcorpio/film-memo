import { useState } from 'react';
import Icon from './Icon.jsx';
import PlatformTag from './PlatformTag.jsx';

const ICON_BASE = '/icon';

/** 分类→迷你占位首字缩写（列表模式海报很小，用单字更清晰） */
const CAT_ABBREV = {
  '电影': '影',
  '电视剧': '剧',
  '综艺': '综',
  '动漫': '动',
  '动画': '动',
  '纪录片': '纪',
  '短片': '短',
};

/** 列表模式小海报：带淡入加载效果 */
function RowPoster({ posterUrl, name, title, category }) {
  const [loaded, setLoaded] = useState(false);
  const abbrev = CAT_ABBREV[category] || (category ? category.slice(0, 1) : '影');
  return (
    <div className="row-poster">
      {posterUrl ? (
        <img
          src={posterUrl}
          alt={title}
          loading="lazy"
          className={loaded ? 'loaded' : ''}
          onLoad={() => setLoaded(true)}
        />
      ) : (
        <div className="poster-placeholder mini">
          <span className="ph-abbrev" title={name}>{abbrev}</span>
        </div>
      )}
    </div>
  );
}

/** ID 单行：SVG 图标 + 可点击 ID 文本（跳转外站），Tag 样式按来源着色 */
function IdLine({ icon, alt, href, value, source }) {
  if (!value) {
    return (
      <span className="row-id-line dim">
        <img src={`${ICON_BASE}/${icon}`} alt="" width={13} height={13} className="row-id-logo" />
        <span className="row-id-value">—</span>
      </span>
    );
  }
  return (
    <a
      className={`row-id-line ${source}`}
      href={href}
      target="_blank"
      rel="noreferrer"
      onClick={(e) => e.stopPropagation()}
      title={`${alt}: ${value}`}
    >
      <img src={`${ICON_BASE}/${icon}`} alt={alt} width={13} height={13} className="row-id-logo" />
      <span className="row-id-value">{value}</span>
    </a>
  );
}

/**
 * 列表模式：表格样式，每部影视一行。
 * 列：海报 / 名称+类别年份集数 / IMDb+豆瓣 / 评分 / 观看平台 / 观看日期
 */
export default function FilmList({ films, onClick, onContextMenu }) {
  return (
    <div className="film-list">
      <table className="film-table">
        <thead>
          <tr>
            <th className="col-poster"></th>
            <th className="col-title">名称</th>
            <th className="col-imdb">IMDb / 豆瓣</th>
            <th className="col-rating">评分</th>
            <th className="col-platforms">观看平台</th>
            <th className="col-date">观看日期</th>
          </tr>
        </thead>
        <tbody>
          {films.map((film) => {
            const meta = film.metadata;
            const title = meta?.title || film.name;
            const year = film.releaseYear || meta?.releaseYear || '';
            return (
              <tr
                key={film.id}
                className="film-row"
                onClick={() => onClick(film)}
                onContextMenu={(e) => onContextMenu(e, film)}
              >
                <td className="col-poster">
                  <RowPoster
                    posterUrl={meta?.posterUrl}
                    name={film.name}
                    title={title}
                    category={film.category}
                  />
                </td>
                <td className="col-title">
                  <div className="row-title" title={title}>{title}</div>
                  <div className="row-sub">
                    <span className="row-cat">{film.category}</span>
                    {year && <span className="row-year">{year}</span>}
                    {film.totalEpisodes > 1 && <span className="row-ep">{film.totalEpisodes} 集</span>}
                  </div>
                </td>
                <td className="col-imdb">
                  <div className="row-id-lines">
                    <IdLine
                      icon="imdb.svg"
                      alt="IMDb"
                      source="imdb"
                      value={film.imdbId}
                      href={`https://www.imdb.com/title/${film.imdbId}`}
                    />
                    <IdLine
                      icon="douban.svg"
                      alt="豆瓣"
                      source="douban"
                      value={film.doubanId}
                      href={`https://movie.douban.com/subject/${film.doubanId}/`}
                    />
                  </div>
                </td>
                <td className="col-rating">
                  {meta?.voteAverage > 0 ? (
                    <span className="row-rating">
                      <Icon name="star" size={12} /> {meta.voteAverage.toFixed(1)}
                    </span>
                  ) : (
                    <span className="row-dim">—</span>
                  )}
                </td>
                <td className="col-platforms">
                  {film.platforms.length > 0 ? (
                    <div className="row-platforms">
                      {film.platforms.map((p) => (
                        <PlatformTag key={p} name={p} size={13} />
                      ))}
                    </div>
                  ) : (
                    <span className="row-dim">—</span>
                  )}
                </td>
                <td className="col-date">
                  {film.startDate ? (
                    film.endDate && film.endDate !== film.startDate ? (
                      <div className="row-date-lines">
                        <span className="row-date-item"><i className="date-tag start">起</i>{film.startDate}</span>
                        <span className="row-date-item"><i className="date-tag end">止</i>{film.endDate}</span>
                      </div>
                    ) : (
                      <div className="row-date-lines">
                        <span className="row-date-item"><i className="date-tag start">起</i>{film.startDate}</span>
                        <span className="row-date-item"><i className="date-tag placeholder">止</i>—</span>
                      </div>
                    )
                  ) : (
                    <span className="row-dim">—</span>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
