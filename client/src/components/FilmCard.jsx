import Icon from './Icon.jsx';
import PlatformTag from './PlatformTag.jsx';

export default function FilmCard({ film, onClick }) {
  const meta = film.metadata;
  const title = meta?.title || film.name;
  const year = film.releaseYear || meta?.releaseYear || '';

  return (
    <div className="film-card" onClick={onClick}>
      <div className="poster">
        {meta?.posterUrl ? (
          <img src={meta.posterUrl} alt={title} loading="lazy" />
        ) : (
          <div className="poster-placeholder">
            <span className="ph-cat">{film.category}</span>
            <span className="ph-name">{film.name}</span>
          </div>
        )}
        {!meta && (
          <span className="no-meta-badge" title="未刮削元数据">
            <Icon name="alert" size={11} /> 无元数据
          </span>
        )}
        {meta?.voteAverage > 0 && (
          <span className="rating-badge" title="评分">
            <Icon name="star" size={12} /> {meta.voteAverage.toFixed(1)}
          </span>
        )}
      </div>
      <div className="card-body">
        <div className="card-line1">
          <span className="card-title" title={title}>{title}</span>
        </div>
        <div className="card-line2">
          <span className="cat-tag">{film.category}</span>
          {year && <span>{year}</span>}
          {film.totalEpisodes > 1 && <span className="ep-text">{film.totalEpisodes} 集</span>}
        </div>
        {film.platforms.length > 0 && (
          <div className="card-platforms">
            {film.platforms.map((p) => (
              <PlatformTag key={p} name={p} size={14} compact />
            ))}
          </div>
        )}
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
