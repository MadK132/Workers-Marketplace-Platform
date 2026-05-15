import { useCallback, useEffect, useRef, useState } from "react";

const FALLBACK_POSITION = {
  latitude: 51.128207,
  longitude: 71.430411,
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
      return Promise.resolve(FALLBACK_POSITION);
    }

    return new Promise((resolve) => {
      navigator.geolocation.getCurrentPosition(
        (result) => {
          const nextPosition = {
          latitude: result.coords.latitude,
          longitude: result.coords.longitude,
          };
          setPosition(nextPosition);
          setGeoStatus("ready");
          resolve(nextPosition);
        },
        () => {
          setPosition(FALLBACK_POSITION);
          setGeoStatus("fallback");
          setGeoError("Location permission was denied. Astana center is used.");
          resolve(FALLBACK_POSITION);
        },
        {
          enableHighAccuracy: true,
          timeout: 8000,
          maximumAge: 30000,
        }
      );
    });
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

    navigator.geolocation.getCurrentPosition(
      (result) => {
        setPosition({
          latitude: result.coords.latitude,
          longitude: result.coords.longitude,
        });
        setGeoStatus("ready");
      },
      () => {
        setGeoStatus("loading");
      },
      {
        enableHighAccuracy: false,
        timeout: 3500,
        maximumAge: 60000,
      }
    );

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
        timeout: 8000,
        maximumAge: 10000,
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
