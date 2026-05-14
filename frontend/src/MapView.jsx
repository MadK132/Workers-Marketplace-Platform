import React, { forwardRef, useEffect, useImperativeHandle, useRef, useState } from "react";
import { loadMapGL } from "./mapgl.js";

const MAP_KEY = import.meta.env.VITE_2GIS_API_KEY || "";

const MapView = forwardRef(function MapView({ position, workers, selectedWorker, onSelectWorker }, ref) {
  const containerRef = useRef(null);
  const mapRef = useRef(null);
  const zoomRef = useRef(13);
  const markersRef = useRef([]);
  const userMarkerRef = useRef(null);
  const [mapError, setMapError] = useState("");

  useImperativeHandle(ref, () => ({
    zoomIn() {
      setMapZoom(zoomRef.current + 1);
    },
    zoomOut() {
      setMapZoom(zoomRef.current - 1);
    },
    recenter() {
      if (mapRef.current && position) {
        mapRef.current.setCenter([position.longitude, position.latitude]);
      }
    },
  }));

  useEffect(() => {
    if (!position || !containerRef.current || mapRef.current || !MAP_KEY) {
      return;
    }

    let cancelled = false;
    loadMapGL()
      .then((mapgl) => {
        if (cancelled) {
          return;
        }
        mapRef.current = new mapgl.Map(containerRef.current, {
          center: [position.longitude, position.latitude],
          zoom: 13,
          key: MAP_KEY,
          controls: [],
        });
        zoomRef.current = 13;
        userMarkerRef.current = new mapgl.Marker(mapRef.current, {
          coordinates: [position.longitude, position.latitude],
          label: {
            text: "You",
          },
        });
      })
      .catch((error) => setMapError(error.message));

    return () => {
      cancelled = true;
    };
  }, [position]);

  useEffect(() => {
    if (!mapRef.current || !window.mapgl || !position) {
      return;
    }

    markersRef.current.forEach((marker) => marker.destroy?.());
    markersRef.current = workers.map((worker) => {
      const marker = new window.mapgl.Marker(mapRef.current, {
        coordinates: [worker.longitude, worker.latitude],
        label: {
          text: `${worker.full_name} - ${formatDistance(worker.distance_meters)}`,
        },
      });
      marker.on("click", () => onSelectWorker(worker));
      return marker;
    });

    if (workers.length > 0) {
      const first = selectedWorker || workers[0];
      mapRef.current.setCenter([first.longitude, first.latitude]);
      setMapZoom(14);
    } else {
      mapRef.current.setCenter([position.longitude, position.latitude]);
      setMapZoom(13);
    }
  }, [workers, selectedWorker, position, onSelectWorker]);

  return (
    <section className="mapShell" aria-label="2GIS map with nearby workers">
      <div ref={containerRef} className="mapCanvas" />
      {!MAP_KEY && (
        <div className="mapOverlay">
          <strong>2GIS API key is missing</strong>
          <span>Set VITE_2GIS_API_KEY in frontend/.env to enable the map.</span>
        </div>
      )}
      {mapError && (
        <div className="mapOverlay">
          <strong>Map is unavailable</strong>
          <span>{mapError}</span>
        </div>
      )}
    </section>
  );

  function setMapZoom(nextZoom) {
    const zoom = Math.max(2, Math.min(20, nextZoom));
    zoomRef.current = zoom;
    mapRef.current?.setZoom(zoom);
  }
});

export default MapView;

function formatDistance(value) {
  if (!Number.isFinite(value)) {
    return "";
  }
  if (value >= 1000) {
    return `${(value / 1000).toFixed(1)} km`;
  }
  return `${Math.round(value)} m`;
}
