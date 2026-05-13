import React from "react";

export default function WorkerList({
  workers,
  selectedWorker,
  onSelectWorker,
  onHireWorker,
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
        const active = selectedWorker?.worker_id === worker.worker_id;
        return (
          <article
            key={worker.worker_id}
            className={active ? "workerCard active" : "workerCard"}
            onClick={() => onSelectWorker(worker)}
          >
            <div>
              <h3>{worker.full_name}</h3>
              <p>{worker.category_name}</p>
            </div>
            <div className="workerMeta">
              <span>{formatDistance(worker.distance_meters)}</span>
              <span>{worker.experience_level}</span>
              <span>{worker.price} KZT</span>
            </div>
            <button type="button" onClick={(event) => onHireClick(event, worker, onHireWorker)}>
              Choose worker
            </button>
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

function formatDistance(value) {
  if (!Number.isFinite(value)) {
    return "nearby";
  }
  if (value >= 1000) {
    return `${(value / 1000).toFixed(1)} km`;
  }
  return `${Math.round(value)} m`;
}
