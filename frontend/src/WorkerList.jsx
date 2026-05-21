import React from "react";

export default function WorkerList({
  workers,
  selectedWorker,
  onSelectWorker,
  onHireWorker,
  onOpenProfile,
  loading,
}) {
  if (loading) {
    return (
      <div className="emptyState">
        <div className="loader" />
        <p>Searching nearby verified workers...</p>
      </div>
    );
  }

  if (workers.length === 0) {
    return (
      <div className="emptyState">
        <strong>No workers yet</strong>
        <p>Choose a category and search from your current location.</p>
      </div>
    );
  }

  return (
    <div className="workerList">
      {workers.map((worker) => {
        const workerID = worker.worker_id || worker.worker_profile_id;
        const active = selectedWorker?.worker_id === worker.worker_id;
        return (
          <article
            key={workerID}
            className={active ? "workerCard active" : "workerCard"}
            onClick={() => onSelectWorker(worker)}
          >
            <div>
              <h3>{worker.full_name}</h3>
              <p>{worker.category_name}</p>
            </div>
            <div className="ratingLine">
              <span>{stars(worker.average_rating)}</span>
              <small>{reviewSummary(worker)}</small>
            </div>
            <div className="workerMeta">
              <span>{formatDistance(worker.distance_meters)}</span>
              <span>{worker.experience_level}</span>
            </div>
            <div className="rowActions">
              <button type="button" onClick={(event) => onHireClick(event, worker, onHireWorker)}>
                Chat
              </button>
              <button className="secondaryButton" type="button" onClick={(event) => onProfileClick(event, worker, onOpenProfile)}>
                Profile
              </button>
            </div>
          </article>
        );
      })}
    </div>
  );
}

function onHireClick(event, worker, onHireWorker) {
  event.stopPropagation();
  onHireWorker(worker);
}

function onProfileClick(event, worker, onOpenProfile) {
  event.stopPropagation();
  onOpenProfile?.(worker);
}

function formatDistance(value) {
  if (!Number.isFinite(value)) {
    return "nearby";
  }
  if (value >= 1000) {
    return `${(value / 1000).toFixed(1)} km`;
  }
  return `${Math.round(value)} m`;
}

function stars(value) {
  const rating = Math.round(Number(value) || 0);
  return Array.from({ length: 5 }, (_, index) => String.fromCharCode(index < rating ? 9733 : 9734)).join("");
}

function reviewSummary(worker) {
  const count = Number(worker.review_count || 0);
  if (!count) return "No reviews yet";
  const rating = Number(worker.average_rating || 0).toFixed(1);
  return `${rating} from ${count} review${count === 1 ? "" : "s"}`;
}
