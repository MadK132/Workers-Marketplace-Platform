import React, { useState } from "react";
import { apiGet } from "./api.js";

export default function WorkerList({
  workers,
  selectedWorker,
  onSelectWorker,
  onHireWorker,
  loading,
  token,
}) {
  const [reviewWorkerID, setReviewWorkerID] = useState(null);
  const [reviewData, setReviewData] = useState(null);
  const [reviewError, setReviewError] = useState("");

  const showReviews = async (event, worker) => {
    event.stopPropagation();
    const workerID = worker.worker_id || worker.worker_profile_id;
    if (reviewWorkerID === workerID) {
      setReviewWorkerID(null);
      setReviewData(null);
      return;
    }
    setReviewWorkerID(workerID);
    setReviewError("");
    setReviewData(null);
    try {
      const data = await apiGet(`/api/reviews/workers/${workerID}`, token);
      setReviewData(data);
    } catch (err) {
      setReviewError(err.message);
    }
  };

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
            key={worker.worker_id}
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
              <button className="secondaryButton" type="button" onClick={(event) => showReviews(event, worker)}>
                Reviews
              </button>
            </div>
            {reviewWorkerID === workerID && (
              <WorkerReviews data={reviewData} error={reviewError} />
            )}
          </article>
        );
      })}
    </div>
  );
}

function WorkerReviews({ data, error }) {
  if (error) return <p className="errorMessage">{error}</p>;
  if (!data) return <p className="muted">Loading reviews...</p>;
  const reviews = data.reviews || [];
  if (reviews.length === 0) return <p className="muted">No reviews yet.</p>;
  return (
    <div className="reviewPreviewList">
      {reviews.slice(0, 3).map((review) => (
        <div className="reviewPreview" key={review.review_id}>
          <strong>{stars(review.rating)} {review.customer_name || "Customer"}</strong>
          <span>{review.category_name}</span>
          {review.comment && <p>{review.comment}</p>}
        </div>
      ))}
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

function stars(value) {
  const rating = Math.round(Number(value) || 0);
  return "★★★★★".split("").map((star, index) => index < rating ? star : "☆").join("");
}

function reviewSummary(worker) {
  const count = Number(worker.review_count || 0);
  if (!count) return "No reviews yet";
  const rating = Number(worker.average_rating || 0).toFixed(1);
  return `${rating} from ${count} review${count === 1 ? "" : "s"}`;
}
