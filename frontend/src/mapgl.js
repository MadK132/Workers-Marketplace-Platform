let loadingPromise;

export function loadMapGL() {
  if (window.mapgl) {
    return Promise.resolve(window.mapgl);
  }
  if (loadingPromise) {
    return loadingPromise;
  }

  loadingPromise = new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = "https://mapgl.2gis.com/api/js/v1";
    script.async = true;
    script.onload = () => resolve(window.mapgl);
    script.onerror = () => reject(new Error("2GIS MapGL failed to load"));
    document.head.appendChild(script);
  });

  return loadingPromise;
}
