import { useEffect, useState, useRef, useCallback } from "react";
import { createRoot } from "react-dom/client";
import { Map } from "react-map-gl";
import maplibregl from "maplibre-gl";
import DeckGL from "@deck.gl/react";
import { ScenegraphLayer } from "@deck.gl/mesh-layers";

const ANIMATIONS = {
  "*": { speed: 1 },
};

const INITIAL_VIEW_STATE = {
  longitude: -0.341004,
  latitude: 51.477487,
  //latitude: 40.641312,
  //longitude: -73.778137,
  //longitude: -84.430464,
  //latitude: 33.640328,
  zoom: 10.852,
  minZoom: 1,
  maxZoom: 18,
  pitch: 37.92255207,
  bearing: 2.394702,
};

const MAP_STYLE =
  "https://basemaps.cartocdn.com/gl/dark-matter-nolabels-gl-style/style.json";

const MODEL_URL =
  "https://raw.githubusercontent.com/emanuelef/glb-models/main/airplane.glb";

export default function App({ sizeScale = 125, mapStyle = MAP_STYLE }) {
  const [data, setData] = useState(null);
  const currentSSE = useRef(null);
  const newPosition = useRef(true);
  const [viewState, setViewState] = useState(INITIAL_VIEW_STATE);

  const setPosition = (lat, lon) => {
    setViewState({
      ...INITIAL_VIEW_STATE,
      longitude: lon,
      latitude: lat,
      zoom: 12,
    });
  };

  function startSSE() {
    const sse = new EventSource(`http://localhost:8080/sse?query=7`);
    if (currentSSE.current) {
      console.log("STOP SSE");
      currentSSE.current.close();
    }
    currentSSE.current = sse;

    console.log("Start SSE");

    sse.onerror = (err) => {
      console.log("on error", err);
    };

    // The onmessage handler is called if no event name is specified for a message.
    sse.onmessage = (msg) => {
      console.log("on message", msg);
    };

    sse.onopen = (...args) => {
      console.log("on open", args);
    };

    sse.addEventListener("current-value", (event) => {
      const parsedData = JSON.parse(event.data);
      const currentValue = parsedData.data;
      console.log(currentValue);

      if (newPosition.current) {
        setPosition(currentValue.lat, currentValue.lon);
        newPosition.current = false;
      }

      setData([currentValue]);
    });
  }

  const goToLocation = useCallback(() => {
    setPosition(-74.1, 40.7);
  }, []);

  useEffect(() => {
    startSSE();
  }, []);

  const layer =
    data &&
    new ScenegraphLayer({
      id: "scenegraph-layer",
      data,
      pickable: true,
      sizeScale,
      scenegraph: MODEL_URL,
      _animations: ANIMATIONS,
      sizeMinPixels: 0.1,
      sizeMaxPixels: 1.5,
      getPosition: (d) => [d.lon || 0, d.lat || 0, d.alt || 0],
    });

  return (
    <div>
      <DeckGL layers={[layer]} initialViewState={viewState} controller={true}>
        <Map
          reuseMaps
          mapLib={maplibregl}
          mapStyle={mapStyle}
          preventStyleDiffing={true}
        />
      </DeckGL>
      <button onClick={goToLocation}>New York City</button>
    </div>
  );
}

export function renderToDOM(container) {
  createRoot(container).render(<App />);
}
