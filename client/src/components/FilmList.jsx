import Icon from './Icon.jsx';
import PlatformTag from './PlatformTag.jsx';

/**
 * 列表模式：表格样式，每部影视一行。
 * 列：海报 / 名称+类别年份集数 / IMDb号 / 评分 / 观看平台 / 观看日期
 */
export default function FilmList({ films, onClick, onContextMenu }) {
  return (
    <div className="film-list">
      <table className="film-table">
        <thead>
          <tr>
            <th className="col-poster"></th>
            <th className="col-title">名称</th>
            <th className="col-imdb">IMDb</th>
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
                  <div className="row-poster">
                    {meta?.posterUrl ? (
                      <img src={meta.posterUrl} alt={title} loading="lazy" />
                    ) : (
                      <div className="poster-placeholder mini">
                        <span className="ph-name">{film.name}</span>
                      </div>
                    )}
                  </div>
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
                  {film.imdbId ? (
                    <a
                      className="row-imdb"
                      href={`https://www.imdb.com/title/${film.imdbId}`}
                      target="_blank"
                      rel="noreferrer"
                      onClick={(e) => e.stopPropagation()}
                    >
                      {film.imdbId} <Icon name="external" size={11} />
                    </a>
                  ) : (
                    <span className="row-dim">—</span>
                  )}
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
