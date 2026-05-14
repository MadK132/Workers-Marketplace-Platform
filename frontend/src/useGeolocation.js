import { useCallback, useEffect, useRef, useState } from "react";

const FALLBACK_POSITION = {
  latitude: 43.238949,
  longitude: 76.889709,
};

export function useGeolocation() {
  const watchID = useRef(null);
  const [position, setPosition] = useState(null);
  const [geoStatus, setGeoStatus] = useState("idle");
  const [geoError, setGeoError] = useState("");

  const locate = useCallback(() => {
    setGeoStatus("loading");
    setGeoError("");

    if (!navigator.geolocation) {
      setPosition(FALLBACK_POSITION);
      setGeoStatus("fallback");
      setGeoError("Browser geolocation is unavailable.");
      return;
    }

    navigator.geolocation.getCurrentPosition(
      (result) => {
        setPosition({
          latitude: result.coords.latitude,
          longitude: result.coords.longitude,
        });
        setGeoStatus("ready");
      },
      () => {
        setPosition(FALLBACK_POSITION);
        setGeoStatus("fallback");
        setGeoError("Location permission was denied. Almaty center is used.");
      },
      {
        enableHighAccuracy: true,
        timeout: 8000,
        maximumAge: 30000,
      }
    );
  }, []);

  const startWatch = useCallback(() => {
    setGeoStatus("loading");
    setGeoError("");

    if (!navigator.geolocation) {
      setGeoStatus("error");
      setGeoError("Browser geolocation is unavailable.");
      return;
    }

    if (watchID.current !== null) {
      navigator.geolocation.clearWatch(watchID.current);
    }

    watchID.current = navigator.geolocation.watchPosition(
      (result) => {
        setPosition({
          latitude: result.coords.latitude,
          longitude: result.coords.longitude,
        });
        setGeoStatus("watching");
      },
      () => {
        setGeoStatus("error");
        setGeoError("Location permission is required for worker mode.");
      },
      {
        enableHighAccuracy: true,
        timeout: 10000,
        maximumAge: 5000,
      }
    );
  }, []);

  const stopWatch = useCallback(() => {
    if (watchID.current !== null && navigator.geolocation) {
      navigator.geolocation.clearWatch(watchID.current);
      watchID.current = null;
    }
  }, []);

  useEffect(() => stopWatch, [stopWatch]);

  return { position, geoStatus, geoError, locate, startWatch, stopWatch };
}
