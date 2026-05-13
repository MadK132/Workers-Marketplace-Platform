import { useCallback, useState } from "react";

const FALLBACK_POSITION = {
  latitude: 43.238949,
  longitude: 76.889709,
};

export function useGeolocation() {
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

  return { position, geoStatus, geoError, locate };
}
