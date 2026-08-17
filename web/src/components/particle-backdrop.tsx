import { useEffect, useRef } from "react";
import CanvasParticles from "canvasparticles-js";

// Dark, blue-leaning gradient the particles drift over (the "connect distance"
// showcase look, tuned darker and cooler). The host div carries the gradient so
// it also renders statically under prefers-reduced-motion.
// const BACKGROUND = "linear-gradient(100deg, #274e7d, #0e1c3a 150%)";
const BACKGROUND = "linear-gradient(100deg, #fafbfd, #3a3b3d 150%)";

// ParticleBackdrop renders a fixed, pointer-transparent canvas of sparse,
// light-colored particles linked by long, faint connecting lines over a dark
// blue gradient. The glass surfaces blur this into frosted material behind them.
// Under prefers-reduced-motion only the static gradient is shown.
export function ParticleBackdrop() {
  const hostRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;

    const canvas = document.createElement("canvas");
    canvas.className = "absolute inset-0 h-full w-full";
    host.appendChild(canvas);

    const engine = new CanvasParticles(canvas).start()

    // const engine = new CanvasParticles(canvas, {
    //   // The host div carries the gradient; the canvas stays transparent.
    //   background: false,
    //   // Canvases are pointer-transparent, so mouse interaction never applies.
    //   mouse: { interactionType: 0 },
    //   particles: {
    //     color: "#dfe9ff",
    //     // Sparse: a few hundred dots per screen, linked by long faint lines.
    //     ppm: 140,
    //     max: 450,
    //     connectDistance: 400,
    //     relSpeed: 0.5,
    //     relSize: 1.2,
    //   },
    // }).start();

    // destroy() stops the animation loop and removes the canvas element.
    return () => engine.destroy();
  }, []);

  return (
    <div
      ref={hostRef}
      className="pointer-events-none fixed inset-0 z-0 overflow-hidden"
      style={{ background: BACKGROUND }}
      aria-hidden="true"
    />
  );
}