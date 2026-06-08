import React, { forwardRef, useEffect, useImperativeHandle, useRef, useState } from "react";
import { loadMapGL } from "./mapgl.js";

const MAP_KEY = import.meta.env.VITE_2GIS_API_KEY || "";

function driverMarkerIcon(rotation = 0) {
  const angle = Number.isFinite(Number(rotation)) ? Number(rotation) : 0;
  return `data:image/svg+xml;charset=UTF-8,${encodeURIComponent(`
<svg width="52" height="52" viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg">
  <g transform="rotate(${angle} 32 32)">
    <path d="M32 8L50 56L32 43L14 56L32 8Z" fill="#FFC21A" stroke="#24302C" stroke-width="2.6" stroke-linejoin="round"/>
    <path d="M32 8L32 43" stroke="#F7A900" stroke-width="1.8"/>
  </g>
</svg>
`)}`;
}

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
  routeLine = null,
  routeFocusKey = "",
  followPosition = false,
  navigationMode = false,
  onUserControl,
}, ref) {
  const containerRef = useRef(null);
  const mapRef = useRef(null);
  const zoomRef = useRef(13);
  const markersRef = useRef([]);
  const userMarkerRef = useRef(null);
  const pickedMarkerRef = useRef(null);
  const routeLineRef = useRef(null);
  const driverBearingRef = useRef(0);
  const previousDriverPositionRef = useRef(null);
  const pickModeRef = useRef(pickMode);
  const onPickPositionRef = useRef(onPickPosition);
  const onUserControlRef = useRef(onUserControl);
  const userAdjustedMapRef = useRef(false);
  const programmaticCameraRef = useRef(false);
  const programmaticCameraTimerRef = useRef(null);
  const centeredWorkersRef = useRef("");
  const focusedRouteRef = useRef("");
  const navigationCameraKeyRef = useRef("");
  const [mapError, setMapError] = useState("");
  const [mapReady, setMapReady] = useState(false);

  useEffect(() => {
    pickModeRef.current = pickMode;
    onPickPositionRef.current = onPickPosition;
    onUserControlRef.current = onUserControl;
  }, [onPickPosition, onUserControl, pickMode]);

  useImperativeHandle(ref, () => ({
    zoomIn() {
      userAdjustedMapRef.current = true;
      onUserControlRef.current?.();
      setMapZoom(zoomRef.current + 1);
    },
    zoomOut() {
      userAdjustedMapRef.current = true;
      onUserControlRef.current?.();
      setMapZoom(zoomRef.current - 1);
    },
    recenter() {
      if (mapRef.current && position) {
        userAdjustedMapRef.current = false;
        runProgrammaticCamera(() => {
          mapRef.current.setCenter([position.longitude, position.latitude]);
          setMapZoom(Math.max(zoomRef.current, 15));
        });
      }
    },
    follow() {
      if (mapRef.current && position) {
        userAdjustedMapRef.current = false;
        if (navigationMode) {
          const nextBearing = driverBearing(position, previousDriverPositionRef.current, routeLine, driverBearingRef.current);
          setNavigationCamera(position, nextBearing, true);
        } else {
          runProgrammaticCamera(() => {
            mapRef.current.setCenter([position.longitude, position.latitude]);
            setMapZoom(Math.max(zoomRef.current, 17));
          });
        }
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
        setMapReady(true);
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
        ["dragstart", "movestart", "zoomstart", "rotatestart", "pitchstart"].forEach((eventName) => {
          mapRef.current.on(eventName, () => {
            if (programmaticCameraRef.current) {
              return;
            }
            userAdjustedMapRef.current = true;
            onUserControlRef.current?.();
          });
        });
        zoomRef.current = 13;
      })
      .catch((error) => setMapError(error.message));

    return () => {
      cancelled = true;
      if (programmaticCameraTimerRef.current) {
        window.clearTimeout(programmaticCameraTimerRef.current);
        programmaticCameraTimerRef.current = null;
      }
    };
  }, [position]);

  useEffect(() => {
    if (!mapReady || !mapRef.current || !window.mapgl) {
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
    if (!userAdjustedMapRef.current) {
      mapRef.current.setCenter([pickedPosition.longitude, pickedPosition.latitude]);
    }

    return () => {
      pickedMarkerRef.current?.destroy?.();
      pickedMarkerRef.current = null;
    };
  }, [mapReady, pickedPosition]);

  useEffect(() => {
    if (!mapReady || !mapRef.current || !window.mapgl) {
      return;
    }
    routeLineRef.current?.destroy?.();
    routeLineRef.current = null;

    const points = Array.isArray(routeLine) ? routeLine : [];
    if (points.length < 2 || !window.mapgl.Polyline) {
      return;
    }

    routeLineRef.current = new window.mapgl.Polyline(mapRef.current, {
      coordinates: points.map((point) => [point.longitude, point.latitude]),
      width: navigationMode ? 7 : 5,
      color: navigationMode ? "#2e8cff" : "#2e8979",
      zIndex: 7,
    });

    const lastPoint = points[points.length - 1];
    const focusKey = routeFocusKey || `${points[0].latitude}:${points[0].longitude}:${lastPoint.latitude}:${lastPoint.longitude}`;
    if (!followPosition && focusedRouteRef.current !== focusKey) {
      focusRoute(points);
      focusedRouteRef.current = focusKey;
    }

    return () => {
      routeLineRef.current?.destroy?.();
      routeLineRef.current = null;
    };
  }, [followPosition, mapReady, navigationMode, routeFocusKey, routeLine]);

  useEffect(() => {
    if (!mapReady || !mapRef.current || navigationMode) {
      return;
    }
    navigationCameraKeyRef.current = "";
    if (typeof mapRef.current.setRotation === "function") {
      mapRef.current.setRotation(0);
    }
    if (typeof mapRef.current.setPitch === "function") {
      mapRef.current.setPitch(0);
    }
  }, [mapReady, navigationMode]);

  useEffect(() => {
    if (!mapReady || !mapRef.current || !position || !followPosition) {
      return;
    }
    userAdjustedMapRef.current = false;
    if (navigationMode) {
      const nextBearing = driverBearing(position, previousDriverPositionRef.current, routeLine, driverBearingRef.current);
      setNavigationCamera(position, nextBearing);
      return;
    }
    runProgrammaticCamera(() => {
      mapRef.current.setCenter([position.longitude, position.latitude]);
      if (zoomRef.current < 17) {
        setMapZoom(17);
      }
    });
  }, [followPosition, mapReady, navigationMode, position?.latitude, position?.longitude, routeLine]);

  useEffect(() => {
    if (!mapReady || !mapRef.current || !window.mapgl || !position) {
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
      const nextBearing = driverBearing(position, previousDriverPositionRef.current, routeLine, driverBearingRef.current);
      driverBearingRef.current = nextBearing;
      previousDriverPositionRef.current = position;
      const mapRotatesWithDriver = navigationMode && followPosition && typeof mapRef.current.setRotation === "function";
      markerOptions.icon = driverMarkerIcon(mapRotatesWithDriver ? 0 : nextBearing);
      markerOptions.size = [52, 52];
      markerOptions.anchor = [26, 26];
      markerOptions.zIndex = 10;
    } else if (userMarker === "default") {
      markerOptions.label = { text: "You" };
    }

    userMarkerRef.current = new window.mapgl.Marker(mapRef.current, markerOptions);

    return () => {
      userMarkerRef.current?.destroy?.();
      userMarkerRef.current = null;
    };
  }, [followPosition, mapReady, navigationMode, position, routeLine, userMarker]);

  useEffect(() => {
    if (!mapReady || !mapRef.current || !window.mapgl || !position) {
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
      const workerKey = `${first.worker_id || ""}:${first.latitude}:${first.longitude}`;
      if (!userAdjustedMapRef.current && centeredWorkersRef.current !== workerKey) {
        mapRef.current.setCenter([first.longitude, first.latitude]);
        setMapZoom(Math.max(zoomRef.current, 14));
        centeredWorkersRef.current = workerKey;
      }
    } else if (autoCenterOnPosition) {
      if (!userAdjustedMapRef.current) {
        mapRef.current.setCenter([position.longitude, position.latitude]);
        setMapZoom(zoomRef.current || 13);
      }
    }
  }, [workers, selectedWorker, position, onSelectWorker, autoCenterOnPosition, mapReady]);

  return (
    <section className={navigationMode ? "mapShell navigationMap" : "mapShell"} aria-label="2GIS map with nearby workers">
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

  function setNavigationCamera(nextPosition, nextBearing, force = false) {
    if (!mapRef.current || !nextPosition) {
      return;
    }
    const bearing = Number.isFinite(Number(nextBearing)) ? Number(nextBearing) : 0;
    const cameraKey = `${roundCoord(nextPosition.latitude)}:${roundCoord(nextPosition.longitude)}:${Math.round(bearing)}`;
    if (!force && navigationCameraKeyRef.current === cameraKey) {
      return;
    }
    navigationCameraKeyRef.current = cameraKey;
    const center = navigationCenter(nextPosition, bearing);
    runProgrammaticCamera(() => {
      mapRef.current.setCenter([center.longitude, center.latitude]);
      setMapZoom(Math.max(zoomRef.current, 18.2));
      if (typeof mapRef.current.setRotation === "function") {
        mapRef.current.setRotation(bearing);
      }
      if (typeof mapRef.current.setPitch === "function") {
        mapRef.current.setPitch(45);
      }
    });
  }

  function runProgrammaticCamera(action) {
    programmaticCameraRef.current = true;
    if (programmaticCameraTimerRef.current) {
      window.clearTimeout(programmaticCameraTimerRef.current);
    }
    action();
    programmaticCameraTimerRef.current = window.setTimeout(() => {
      programmaticCameraRef.current = false;
      programmaticCameraTimerRef.current = null;
    }, 350);
  }

  function focusRoute(points) {
    if (!mapRef.current || points.length < 2) {
      return;
    }
    const midPoint = points[Math.floor(points.length / 2)];
    if (!midPoint) {
      return;
    }
    mapRef.current.setCenter([midPoint.longitude, midPoint.latitude]);
    setMapZoom(Math.max(zoomRef.current, 16));
  }
});

export default MapView;

function navigationCenter(position, bearing) {
  const lookAhead = destinationPoint(position, bearing, 40);
  if (!lookAhead) {
    return position;
  }
  return lookAhead;
}

function destinationPoint(origin, bearing, meters) {
  if (!origin) return null;
  const radius = 6371000;
  const angularDistance = Number(meters) / radius;
  const bearingRad = degreesToRadians(Number(bearing));
  const lat1 = degreesToRadians(Number(origin.latitude));
  const lon1 = degreesToRadians(Number(origin.longitude));
  const lat2 = Math.asin(
    Math.sin(lat1) * Math.cos(angularDistance) +
    Math.cos(lat1) * Math.sin(angularDistance) * Math.cos(bearingRad),
  );
  const lon2 = lon1 + Math.atan2(
    Math.sin(bearingRad) * Math.sin(angularDistance) * Math.cos(lat1),
    Math.cos(angularDistance) - Math.sin(lat1) * Math.sin(lat2),
  );
  return {
    latitude: radiansToDegrees(lat2),
    longitude: ((radiansToDegrees(lon2) + 540) % 360) - 180,
  };
}

function roundCoord(value) {
  return Number(value).toFixed(5);
}

function driverBearing(position, previousPosition, routeLine, fallbackBearing) {
  if (previousPosition && metersBetween(previousPosition, position) >= 2) {
    return bearingDegrees(previousPosition, position);
  }

  const routePoints = Array.isArray(routeLine) ? routeLine : [];
  const target = routePoints.find((point) => metersBetween(position, point) >= 8) || routePoints[routePoints.length - 1];
  if (target && Number.isFinite(Number(target.latitude)) && Number.isFinite(Number(target.longitude))) {
    return bearingDegrees(position, target);
  }

  return Number.isFinite(Number(fallbackBearing)) ? Number(fallbackBearing) : 0;
}

function bearingDegrees(from, to) {
  const lat1 = degreesToRadians(Number(from.latitude));
  const lat2 = degreesToRadians(Number(to.latitude));
  const deltaLon = degreesToRadians(Number(to.longitude) - Number(from.longitude));
  const y = Math.sin(deltaLon) * Math.cos(lat2);
  const x = Math.cos(lat1) * Math.sin(lat2) -
    Math.sin(lat1) * Math.cos(lat2) * Math.cos(deltaLon);
  return (radiansToDegrees(Math.atan2(y, x)) + 360) % 360;
}

function metersBetween(a, b) {
  if (!a || !b) return 0;
  const radius = 6371000;
  const lat1 = degreesToRadians(Number(a.latitude));
  const lat2 = degreesToRadians(Number(b.latitude));
  const deltaLat = degreesToRadians(Number(b.latitude) - Number(a.latitude));
  const deltaLon = degreesToRadians(Number(b.longitude) - Number(a.longitude));
  const halfChord = Math.sin(deltaLat / 2) ** 2 +
    Math.cos(lat1) * Math.cos(lat2) * Math.sin(deltaLon / 2) ** 2;
  return 2 * radius * Math.atan2(Math.sqrt(halfChord), Math.sqrt(1 - halfChord));
}

function degreesToRadians(value) {
  return value * Math.PI / 180;
}

function radiansToDegrees(value) {
  return value * 180 / Math.PI;
}
