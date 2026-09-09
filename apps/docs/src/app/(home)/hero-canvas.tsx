"use client";

import type { Locale } from "@/i18n";
import { useEffect, useRef, useState } from "react";
import type * as THREE from "three";
import { createBrandArcGeometry } from "./hero-brand-geometry";
import { createWorkspace, type ModuleId } from "./hero-workspace";

type Three = typeof THREE;
type HeroController = { dispose: () => void };
type HomeHeroCanvasProps = { ariaLabel?: string; lang?: Locale };

const copy = {
  zh: {
    description: "One CLI 品牌双弧归一后生成工作区模块，展示前端、后端、文档、共享库、部署和 CLI 接口模块。",
  },
  en: {
    description: "One CLI brand arcs reunite before the workspace assembles, showing frontend, backend, docs, library, deploy, and CLI interface modules.",
  },
};

export function HomeHeroCanvas({ ariaLabel, lang = "zh" }: HomeHeroCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const controllerRef = useRef<HeroController | null>(null);
  const [ready, setReady] = useState(false);
  const text = copy[lang];

  useEffect(() => {
    const canvas = canvasRef.current;
    const container = canvas?.parentElement;
    if (!canvas || !container) return;
    let disposed = false;
    setReady(false);
    const initialize = async () => {
      try {
        const [THREE, { RoomEnvironment }] = await Promise.all([
          import("three"),
          import("three/addons/environments/RoomEnvironment.js"),
          document.fonts?.ready,
        ]);
        if (disposed) return;
        controllerRef.current = mountScene(THREE, RoomEnvironment, canvas, container, lang);
        setReady(true);
      } catch (error) {
        console.warn("One CLI hero animation could not initialize WebGL.", error);
        if (!disposed) setReady(true);
      }
    };
    void initialize();
    return () => {
      disposed = true;
      controllerRef.current?.dispose();
      controllerRef.current = null;
    };
  }, [lang]);

  return (
    <div className="relative h-[420px] min-w-0 overflow-hidden md:h-[560px] lg:h-[610px]" role="img" aria-label={ariaLabel ?? text.description}>
      {!ready && (
        <svg aria-hidden="true" viewBox="0 0 120 120" className="absolute inset-0 m-auto w-[62%] max-w-[320px]">
          <path d="M 46.32 22.41 A 40 40 0 0 1 97.59 73.68 M 73.68 97.59 A 40 40 0 0 1 22.41 46.32" fill="none" stroke="#ea580c" strokeWidth="18" strokeLinecap="round" />
        </svg>
      )}
      <canvas
        ref={canvasRef}
        aria-hidden="true"
        className="absolute inset-0 block size-full"
        data-one-hero-canvas
      />
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_52%_34%,rgba(255,255,255,0.08),transparent_26%),radial-gradient(circle_at_54%_44%,rgba(234,88,12,0.04),transparent_34%),linear-gradient(180deg,transparent,rgba(10,10,10,0.34))]" />
    </div>
  );
}

function mountScene(
  THREE: Three,
  RoomEnvironment: typeof import("three/addons/environments/RoomEnvironment.js").RoomEnvironment,
  canvas: HTMLCanvasElement,
  container: HTMLElement,
  lang: Locale,
): HeroController {
  const renderer = new THREE.WebGLRenderer({ canvas, alpha: true, antialias: true, powerPreference: "high-performance" });
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.setClearColor(0x000000, 0);
  const motionQuery = window.matchMedia("(prefers-reduced-motion: reduce)");

  const scene = new THREE.Scene();
  const camera = new THREE.OrthographicCamera(-1, 1, 1, -1, -20, 20);
  camera.up.set(0, 1, 0);
  const introCameraPosition = new THREE.Vector3(0, 0.02, 8);
  const introCameraTarget = new THREE.Vector3(0, 0.02, 0.566);
  const baseCameraPosition = new THREE.Vector3(3.35, -5.35, 3.85);
  const baseCameraTarget = new THREE.Vector3(0.02, 0, 0.2);
  const activeCameraPosition = new THREE.Vector3(1.92, -5.08, 4.38);
  const activeCameraTarget = new THREE.Vector3(-0.08, 0.03, 0.32);
  camera.position.copy(introCameraPosition);
  camera.lookAt(introCameraTarget);
  const frontRotation = camera.quaternion.clone();
  const boardRotation = new THREE.Quaternion();
  const cameraTarget = new THREE.Vector3();
  const workspaceCameraPosition = new THREE.Vector3();
  const workspaceCameraTarget = new THREE.Vector3();

  const createEnvironment = () => {
    const room = new RoomEnvironment();
    const pmrem = new THREE.PMREMGenerator(renderer);
    const target = pmrem.fromScene(room, 0.04);
    room.dispose();
    pmrem.dispose();
    return target;
  };
  let environment: ReturnType<typeof createEnvironment> | null = motionQuery.matches ? null : createEnvironment();
  scene.add(new THREE.AmbientLight(0xffffff, 2.05));
  const key = new THREE.DirectionalLight(0xffffff, 3.7);
  key.position.set(-2.6, -3.2, 5.2);
  scene.add(key);
  const rim = new THREE.DirectionalLight(0xfff7ed, 2.9);
  rim.position.set(4.4, -1.8, 3.6);
  scene.add(rim);

  const workspace = createWorkspace(THREE, lang, requestRender);
  scene.add(workspace.group, workspace.amberPulse);
  const brand = new THREE.Group();
  workspace.group.add(brand);
  const geometry = createBrandArcGeometry(THREE);
  const material = new THREE.MeshPhysicalMaterial({
    color: 0xea580c, metalness: 0.62, roughness: 0.31,
    clearcoat: 0.2, clearcoatRoughness: 0.25, envMap: environment?.texture ?? null, envMapIntensity: 0.6,
  });
  const arcs = [-20, 160].map((angle) => {
    const pivot = new THREE.Group();
    const arc = new THREE.Mesh(geometry, material);
    arc.rotation.z = THREE.MathUtils.degToRad(angle);
    pivot.add(arc);
    brand.add(pivot);
    return pivot;
  });
  const brandOrigin = new THREE.Vector3(0.17, 0, 0.65);
  // Match the original 0.18-unit SVG viewport: its centreline radius is 40/120.
  const stampScale = 0.18 / 120 * 40 / 1.24;
  let brandDisposed = false;
  const disposeBrand = () => {
    if (brandDisposed) return;
    brandDisposed = true;
    workspace.group.remove(brand);
    geometry.dispose();
    material.dispose();
    environment?.dispose();
    environment = null;
  };

  const gridPoints: import("three").Vector3[] = [];
  for (let n = -7; n <= 7; n += 0.42) {
    gridPoints.push(new THREE.Vector3(n, -7, -2.6), new THREE.Vector3(n + 2.5, 7, -2.6));
    gridPoints.push(new THREE.Vector3(-7, n, -2.6), new THREE.Vector3(7, n + 1.8, -2.6));
  }
  const gridMaterial = new THREE.LineBasicMaterial({ color: 0x78716c, transparent: true, opacity: 0 });
  const grid = new THREE.LineSegments(new THREE.BufferGeometry().setFromPoints(gridPoints), gridMaterial);
  grid.position.set(0.35, 0.45, 0);
  scene.add(grid);

  const assemblyStart = 2.6;
  let reducedMotion = motionQuery.matches;
  const startedAt = performance.now() - (reducedMotion ? assemblyStart * 1000 : 0);
  let elapsed = 0;
  let activeId: ModuleId | null = null;
  let hoveredId: ModuleId | null = null;
  let activeMix = 0;
  const workspaceFrame = {
    elapsed: 0, pointerX: 0, pointerY: 0, activeMix: 0,
    activeId: null as ModuleId | null, hoveredId: null as ModuleId | null,
    reducedMotion, focusResponse: 1,
  };
  let disposed = false;
  let contextLost = false;
  let inViewport = true;
  let hasSize = false;
  let frame = 0;
  let previousFrameTime: number | null = null;
  let width = 0;
  let height = 0;
  let pixelRatio = 0;
  const pointer = {
    x: 0, y: 0, targetX: 0, targetY: 0,
    id: null as number | null, pressedModule: null as ModuleId | null,
    startX: 0, startY: 0, travel: 0, tapSlop: 10, cancelled: false,
  };
  let hoverPending = false;
  const raycaster = new THREE.Raycaster();
  const pointerNdc = new THREE.Vector2();
  const hoverNdc = new THREE.Vector2();
  const hitTargets: import("three").Object3D[] = [];
  const hitResults: import("three").Intersection[] = [];
  const moduleByHitTarget = new Map(workspace.modules.map((module) => [module.hitTarget, module.config.id]));
  const clamp = (value: number) => THREE.MathUtils.clamp(value, 0, 1);
  const smooth = (from: number, to: number, value: number) => {
    const t = clamp((value - from) / (to - from));
    return t * t * (3 - 2 * t);
  };
  const cubic = (t: number) => t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
  const canRender = () => !disposed && !contextLost && inViewport && hasSize && !document.hidden;

  function requestRender() {
    if (!frame && canRender()) frame = window.requestAnimationFrame(render);
  }
  const stopFrame = () => {
    if (frame) window.cancelAnimationFrame(frame);
    frame = 0;
    previousFrameTime = null;
  };
  const syncPlayback = () => { if (canRender()) requestRender(); else stopFrame(); };
  const resize = () => {
    const rect = container.getBoundingClientRect();
    hasSize = rect.width > 0 && rect.height > 0;
    if (!hasSize || disposed || contextLost) return;
    const nextWidth = Math.max(320, rect.width);
    const nextHeight = Math.max(320, rect.height);
    const nextPixelRatio = Math.min(window.devicePixelRatio || 1, 2);
    if (width === nextWidth && height === nextHeight && pixelRatio === nextPixelRatio) return;
    width = nextWidth;
    height = nextHeight;
    pixelRatio = nextPixelRatio;
    renderer.setPixelRatio(pixelRatio);
    renderer.setSize(width, height, false);
    const aspect = width / height;
    const viewHeight = width < 520 ? 5.04 : 4.84;
    camera.left = -viewHeight * aspect / 2;
    camera.right = viewHeight * aspect / 2;
    camera.top = viewHeight / 2;
    camera.bottom = -viewHeight / 2;
    camera.updateProjectionMatrix();
  };

  function render(now: number) {
    frame = 0;
    if (!canRender()) return;
    const delta = previousFrameTime === null ? 1 / 60 : Math.max(0, now - previousFrameTime) / 1000;
    previousFrameTime = now;
    // Share a time-based response so the camera and detail panel open together at any refresh rate.
    const focusResponse = reducedMotion ? 1 : 1 - Math.exp(-10 * delta);
    // Keep master’s wall-clock animation phase, even while offscreen rendering is suspended.
    elapsed = reducedMotion
      ? assemblyStart + workspace.reducedElapsed / 1000
      : Math.max(0, (now - startedAt) / 1000);
    if (hoverPending) {
      hoveredId = pickAt(hoverNdc);
      hoverPending = false;
      canvas.style.cursor = hoveredId ? "pointer" : "default";
    }
    pointer.x += (pointer.targetX - pointer.x) * 0.08;
    pointer.y += (pointer.targetY - pointer.y) * 0.08;
    activeMix += ((activeId ? 1 : 0) - activeMix) * focusResponse;

    const landing = brandDisposed ? 1 : cubic(smooth(1.85, 2.8, elapsed));
    const cameraMix = cubic(smooth(0, 1, activeMix));
    workspaceFrame.elapsed = reducedMotion ? workspace.reducedElapsed : Math.max(0, now - startedAt) - assemblyStart * 1000;
    workspaceFrame.pointerX = pointer.x;
    workspaceFrame.pointerY = pointer.y;
    workspaceFrame.activeMix = activeMix;
    workspaceFrame.activeId = activeId;
    workspaceFrame.hoveredId = hoveredId;
    workspaceFrame.reducedMotion = reducedMotion;
    workspaceFrame.focusResponse = focusResponse;
    workspace.update(workspaceFrame);

    workspaceCameraPosition.lerpVectors(baseCameraPosition, activeCameraPosition, cameraMix);
    workspaceCameraTarget.lerpVectors(baseCameraTarget, activeCameraTarget, cameraMix);
    camera.position.lerpVectors(introCameraPosition, workspaceCameraPosition, landing);
    cameraTarget.lerpVectors(introCameraTarget, workspaceCameraTarget, landing);
    camera.up.set(0, 1 - landing, landing).normalize();
    camera.lookAt(cameraTarget);

    workspace.printedMarkIcon.visible = brandDisposed || elapsed >= 2.8;
    if (!brandDisposed) {
      if (elapsed >= 2.8) {
        // The introduction hands off to master’s original printed SVG mark.
        // Its geometry and reflection map are no longer needed during the workspace loop.
        disposeBrand();
      } else {
        const separation = 1 - cubic(smooth(0.05, 1.4, elapsed));
        brand.position.lerpVectors(brandOrigin, workspace.launcher, landing);
        brand.position.z += Math.sin(landing * Math.PI) * 0.22;
        brand.scale.setScalar(THREE.MathUtils.lerp(1.12, stampScale, landing));
        brand.quaternion.copy(workspace.group.quaternion).invert().multiply(frontRotation).slerp(boardRotation, landing);
        arcs.forEach((arc, index) => {
          const direction = index === 0 ? 1 : -1;
          arc.position.set(direction * separation * 0.3, direction * separation * 0.09, direction * separation * 0.2);
          arc.rotation.set(direction * separation * 0.6, direction * separation * 0.95, direction * separation * 0.35);
        });
      }
    }
    gridMaterial.opacity = landing * 0.08;
    renderer.render(scene, camera);
    if (!reducedMotion) requestRender();
  }

  function pickAt(ndc: import("three").Vector2) {
    if (!canRender() || elapsed < 2.8) return null;
    camera.updateMatrixWorld();
    workspace.group.updateWorldMatrix(true, true);
    raycaster.setFromCamera(ndc, camera);
    hitTargets.length = 0;
    hitResults.length = 0;
    for (const module of workspace.modules) {
      if (module.group.visible && module.hitTarget.visible) hitTargets.push(module.hitTarget);
    }
    raycaster.intersectObjects(hitTargets, false, hitResults);
    return moduleByHitTarget.get(hitResults[0]?.object as import("three").Mesh) ?? null;
  }

  const pickModule = (event: PointerEvent) => {
    const rect = canvas.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) return null;
    pointerNdc.set((event.clientX - rect.left) / rect.width * 2 - 1, 1 - (event.clientY - rect.top) / rect.height * 2);
    return pickAt(pointerNdc);
  };
  const onPointerDown = (event: PointerEvent) => {
    if (pointer.id !== null) {
      if (pointer.id !== event.pointerId) pointer.cancelled = true;
      return;
    }
    if (!event.isPrimary || event.button !== 0 || !canRender()) return;
    pointer.id = event.pointerId;
    // Keep the card the user pressed even if the camera or page moves before release.
    pointer.pressedModule = pickModule(event);
    pointer.startX = event.clientX;
    pointer.startY = event.clientY;
    pointer.travel = 0;
    pointer.tapSlop = event.pointerType === "touch" ? 14 : 10;
    pointer.cancelled = false;
    canvas.setPointerCapture(event.pointerId);
  };
  const onPointerMove = (event: PointerEvent) => {
    if (!canRender()) return;
    if (pointer.id !== null && pointer.id === event.pointerId) {
      pointer.travel = Math.max(pointer.travel, Math.hypot(event.clientX - pointer.startX, event.clientY - pointer.startY));
    }
    const rect = canvas.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) return;
    pointer.targetX = (event.clientX - rect.left) / rect.width - 0.5;
    pointer.targetY = (event.clientY - rect.top) / rect.height - 0.5;
    hoverNdc.set(pointer.targetX * 2, -pointer.targetY * 2);
    // Multiple pointer events before a frame only need the latest raycast.
    hoverPending = true;
    requestRender();
  };
  const endPointer = (event: PointerEvent) => {
    if (pointer.id !== event.pointerId) return;
    const id = pointer.pressedModule;
    const travel = event.type === "pointerup"
      ? Math.max(pointer.travel, Math.hypot(event.clientX - pointer.startX, event.clientY - pointer.startY))
      : pointer.travel;
    const isTap = event.type === "pointerup" && !pointer.cancelled && travel <= pointer.tapSlop;
    pointer.id = null;
    pointer.pressedModule = null;
    if (isTap && canRender()) {
      activeId = id === activeId ? null : id;
    }
    if (canvas.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId);
    requestRender();
  };
  const onPointerLeave = () => {
    hoverPending = false;
    hoveredId = null;
    pointer.targetX = pointer.targetY = 0;
    canvas.style.cursor = "default";
    requestRender();
  };
  const onMotionChange = () => {
    reducedMotion = motionQuery.matches;
    stopFrame();
    syncPlayback();
  };
  const onContextLost = (event: Event) => { event.preventDefault(); contextLost = true; stopFrame(); };
  const onContextRestored = () => {
    contextLost = false;
    if (!brandDisposed) {
      environment?.dispose();
      environment = createEnvironment();
      material.envMap = environment.texture;
      material.needsUpdate = true;
    }
    width = 0;
    resize();
    syncPlayback();
  };
  const resizeObserver = new ResizeObserver(() => { resize(); syncPlayback(); });
  const intersectionObserver = new IntersectionObserver((entries) => {
    const entry = entries.find((item) => item.target === container);
    if (entry) { inViewport = entry.isIntersecting; syncPlayback(); }
  });
  const events = {
    pointerdown: onPointerDown, pointermove: onPointerMove, pointerup: endPointer,
    pointercancel: endPointer, lostpointercapture: endPointer, pointerleave: onPointerLeave,
    webglcontextlost: onContextLost, webglcontextrestored: onContextRestored,
  };
  for (const [name, listener] of Object.entries(events)) canvas.addEventListener(name, listener as EventListener);
  document.addEventListener("visibilitychange", syncPlayback);
  motionQuery.addEventListener("change", onMotionChange);
  resizeObserver.observe(container);
  intersectionObserver.observe(container);
  resize();
  requestRender();

  return {
    dispose: () => {
      disposed = true;
      stopFrame();
      for (const [name, listener] of Object.entries(events)) canvas.removeEventListener(name, listener as EventListener);
      if (pointer.id !== null && canvas.hasPointerCapture(pointer.id)) canvas.releasePointerCapture(pointer.id);
      document.removeEventListener("visibilitychange", syncPlayback);
      motionQuery.removeEventListener("change", onMotionChange);
      resizeObserver.disconnect();
      intersectionObserver.disconnect();
      canvas.style.cursor = "";
      disposeBrand();
      const geometries = new Set<import("three").BufferGeometry>();
      const materials = new Set<import("three").Material>();
      const textures = new Set<import("three").Texture>();
      scene.traverse((object) => {
        const mesh = object as import("three").Mesh;
        if (mesh.geometry) geometries.add(mesh.geometry);
        if (mesh.material) for (const item of Array.isArray(mesh.material) ? mesh.material : [mesh.material]) materials.add(item);
      });
      materials.forEach((item) => {
        for (const value of Object.values(item)) {
          if (value && typeof value === "object" && "isTexture" in value && value.isTexture) textures.add(value as import("three").Texture);
        }
        item.dispose();
      });
      geometries.forEach((item) => item.dispose());
      textures.forEach((item) => item.dispose());
      renderer.dispose();
    },
  };
}
