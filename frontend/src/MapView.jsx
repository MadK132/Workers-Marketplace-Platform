import React, { forwardRef, useEffect, useImperativeHandle, useRef, useState } from "react";
import { loadMapGL } from "./mapgl.js";

const MAP_KEY = import.meta.env.VITE_2GIS_API_KEY || "";

const DRIVER_MARKER_ICON = `data:image/svg+xml;charset=UTF-8,${encodeURIComponent(`
<svg width="44" height="54" viewBox="0 0 44 54" fill="none" xmlns="http://www.w3.org/2000/svg">
  <path d="M22 2L42 52L22 38L2 52L22 2Z" fill="#FFC21A" stroke="#24302C" stroke-width="2" stroke-linejoin="round"/>
  <path d="M22 2L22 38" stroke="#F7A900" stroke-width="1.5"/>
</svg>
`)}`;

const WORKER_MARKER_ICON = `data:image/svg+xml;charset=UTF-8,${encodeURIComponent(`
<svg width="42" height="48" viewBox="0 0 42 48" fill="none" xmlns="http://www.w3.org/2000/svg">
  <path d="M21 46C21 46 36 30.8 36 18C36 8.6 29.3 3 21 3C12.7 3 6 8.6 6 18C6 30.8 21 46 21 46Z" fill="#2E8979" stroke="#10231F" stroke-width="2"/>
  <circle cx="21" cy="18" r="10" fill="#FFFAF2"/>
  <path d="M15 18.5H27M17 15H25V23H17V15ZM18.5 15V12.8C18.5 11.8 19.3 11 20.3 11H21.7C22.7 11 23.5 11.8 23.5 12.8V15" stroke="#10231F" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
</svg>
`)}`;

const SELECTED_WORKER_MARKER_ICON = `data:image/svg+xml;charset=UTF-8,${encodeURIComponent(`
<svg width="48" height="54" viewBox="0 0 48 54" fill="none" xmlns="http://www.w3.org/2000/svg">
  <path d="M24 52C24 52 42 34.4 42 20C42 9.8 34 3 24 3C14 3 6 9.8 6 20C6 34.4 24 52 24 52Z" fill="#FFD900" stroke="#10231F" stroke-width="2.5"/>
  <circle cx="24" cy="20" r="11" fill="#FFFAF2"/>
  <path d="M17 20.5H31M19.5 16.5H28.5V25H19.5V16.5ZM21 16.5V14C21 12.9 21.9 12 23 12H25C26.1 12 27 12.9 27 14V16.5" stroke="#10231F" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>
</svg>
`)}`;

const MapView = forwardRef(function MapView({
  position,
  workers,
  selectedWorker,
  onSelectWorker,
  userMarker = "default",
  pickMode = false,
  pickedPosition = null,
  onPickPosition,
  autoCenterOnPosition = true,
}, ref) {
  const containerRef = useRef(null);
  const mapRef = useRef(null);
  const zoomRef = useRef(13);
  const markersRef = useRef([]);
  const userMarkerRef = useRef(null);
  const pickedMarkerRef = useRef(null);
  const pickModeRef = useRef(pickMode);
  const onPickPositionRef = useRef(onPickPosition);
  const [mapError, setMapError] = useState("");

  useEffect(() => {
    pickModeRef.current = pickMode;
    onPickPositionRef.current = onPickPosition;
  }, [onPickPosition, pickMode]);

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
          zoomControl: false,
          controls: [],
        });
        mapRef.current.on("click", (event) => {
          const coordinates = event.lngLat || event.coordinates || event.lnglat;
          if (!coordinates || !pickModeRef.current || !onPickPositionRef.current) {
            return;
          }
          const longitude = Array.isArray(coordinates) ? coordinates[0] : coordinates.lng;
          const latitude = Array.isArray(coordinates) ? coordinates[1] : coordinates.lat;
          if (!Number.isFinite(Number(latitude)) || !Number.isFinite(Number(longitude))) {
            return;
          }
          onPickPositionRef.current({
            longitude: Number(longitude),
            latitude: Number(latitude),
          });
        });
        zoomRef.current = 13;
      })
      .catch((error) => setMapError(error.message));

    return () => {
      cancelled = true;
    };
  }, [position]);

  useEffect(() => {
    if (!mapRef.current || !window.mapgl) {
      return;
    }

    pickedMarkerRef.current?.destroy?.();
    pickedMarkerRef.current = null;
    if (!pickedPosition) {
      return;
    }

    pickedMarkerRef.current = new window.mapgl.Marker(mapRef.current, {
      coordinates: [pickedPosition.longitude, pickedPosition.latitude],
      zIndex: 9,
    });

    return () => {
      pickedMarkerRef.current?.destroy?.();
      pickedMarkerRef.current = null;
    };
  }, [pickedPosition]);

  useEffect(() => {
    if (!mapRef.current || !window.mapgl || !position) {
      return;
    }

    userMarkerRef.current?.destroy?.();
    userMarkerRef.current = null;

    if (userMarker === "none") {
      return;
    }

    const markerOptions = {
      coordinates: [position.longitude, position.latitude],
    };

    if (userMarker === "driver") {
      markerOptions.icon = DRIVER_MARKER_ICON;
      markerOptions.size = [44, 54];
      markerOptions.anchor = [22, 27];
      markerOptions.zIndex = 10;
    } else if (userMarker === "default") {
      markerOptions.label = { text: "You" };
    }

    userMarkerRef.current = new window.mapgl.Marker(mapRef.current, markerOptions);

    return () => {
      userMarkerRef.current?.destroy?.();
      userMarkerRef.current = null;
    };
  }, [position, userMarker]);

  useEffect(() => {
    if (!mapRef.current || !window.mapgl || !position) {
      return;
    }

    markersRef.current.forEach((marker) => marker.destroy?.());
    markersRef.current = workers.map((worker) => {
      const active = selectedWorker?.worker_id === worker.worker_id;
      const marker = new window.mapgl.Marker(mapRef.current, {
        coordinates: [worker.longitude, worker.latitude],
        icon: active ? SELECTED_WORKER_MARKER_ICON : WORKER_MARKER_ICON,
        size: active ? [48, 54] : [42, 48],
        anchor: active ? [24, 52] : [21, 46],
        zIndex: active ? 12 : 8,
      });
      marker.on("click", () => onSelectWorker(worker));
      return marker;
    });

    if (workers.length > 0) {
      const first = selectedWorker || workers[0];
      mapRef.current.setCenter([first.longitude, first.latitude]);
      setMapZoom(14);
    } else if (autoCenterOnPosition) {
      mapRef.current.setCenter([position.longitude, position.latitude]);
      setMapZoom(13);
    }
  }, [workers, selectedWorker, position, onSelectWorker, autoCenterOnPosition]);

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
