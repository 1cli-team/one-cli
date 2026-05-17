"use client";

import type { Locale } from "@/i18n";
import { useEffect, useRef } from "react";
import * as THREE from "three";

type Group = import("three").Group;
type Line = import("three").Line;
type Material = import("three").Material;
type Mesh = import("three").Mesh;
type Object3D = import("three").Object3D;
type Texture = import("three").Texture;
type Vector3 = import("three").Vector3;

type ModuleConfig = {
  labels: Record<Locale, string>;
  x: number;
  y: number;
  width: number;
  depth: number;
  delay: number;
};

type ModuleModel = {
  config: ModuleConfig;
  group: Group;
  labelMaterials: Material[];
  materials: Material[];
  route: Line;
  routeMaterial: Material;
  start: Vector3;
  control: Vector3;
  hoverTarget: Vector3;
  target: Vector3;
};

type PrintedMarkModel = {
  group: Group;
  labelMaterials: Material[];
  materials: Material[];
};

type BoardModel = {
  group: Group;
  materials: Material[];
  flowMaterial: Material;
  flowTexture: Texture;
};

type HomeHeroCanvasProps = {
  ariaLabel?: string;
  lang?: Locale;
};

type Three = typeof THREE;

const moduleConfigs: ModuleConfig[] = [
  { labels: { zh: "前端", en: "Frontend" }, x: -1.34, y: -0.82, width: 0.76, depth: 0.44, delay: 0 },
  { labels: { zh: "后端", en: "Backend" }, x: -0.45, y: -0.82, width: 0.76, depth: 0.44, delay: 300 },
  { labels: { zh: "桌面", en: "Desktop" }, x: 0.44, y: -0.82, width: 0.76, depth: 0.44, delay: 600 },
  { labels: { zh: "移动", en: "Mobile" }, x: -1.34, y: -0.25, width: 0.72, depth: 0.44, delay: 900 },
  { labels: { zh: "文档", en: "Docs" }, x: -0.45, y: -0.25, width: 0.72, depth: 0.44, delay: 1200 },
  { labels: { zh: "共享库", en: "Library" }, x: 0.44, y: -0.25, width: 0.88, depth: 0.44, delay: 1500 },
  { labels: { zh: "环境", en: "Env" }, x: 1.34, y: -0.25, width: 0.72, depth: 0.44, delay: 1800 },
  { labels: { zh: "容器", en: "Container" }, x: -1.34, y: 0.34, width: 0.72, depth: 0.44, delay: 2100 },
  { labels: { zh: "部署", en: "Deploy" }, x: -0.45, y: 0.34, width: 0.72, depth: 0.44, delay: 2400 },
  { labels: { zh: "JSON", en: "JSON" }, x: 0.44, y: 0.34, width: 0.72, depth: 0.44, delay: 2700 },
  { labels: { zh: "错误恢复", en: "Recovery" }, x: 1.34, y: 0.34, width: 0.92, depth: 0.44, delay: 3000 },
  { labels: { zh: "AI上下文", en: "AI Context" }, x: -1.24, y: 0.94, width: 0.96, depth: 0.44, delay: 3300 },
];

const boardWidth = 3.82;
const boardDepth = 2.78;
const boardHeight = 0.18;
const boardTopZ = boardHeight - 0.08;
const launcherX = 1.24;
const launcherY = -0.96;
const launcherLift = 0.09;
const moduleHeight = 0.24;
const coreStart = 120;
const moduleStart = 320;
const moduleDuration = 1080;
const settleStart = moduleStart + moduleConfigs[moduleConfigs.length - 1].delay + moduleDuration;
const sceneScale = 0.84;

export function HomeHeroCanvas({ ariaLabel, lang = "zh" }: HomeHeroCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const container = canvas?.parentElement;

    if (!canvas || !container) {
      return;
    }

    let disposeScene: (() => void) | undefined;
    let disposed = false;

    const initialize = async () => {
      await document.fonts?.ready;

      if (disposed) {
        return;
      }

      try {
        disposeScene = mountScene(THREE, canvas, container, lang);
      } catch (error) {
        console.warn("One CLI hero animation could not initialize WebGL.", error);
      }
    };

    void initialize();

    return () => {
      disposed = true;
      disposeScene?.();
    };
  }, [lang]);

  return (
    <div
      aria-label={
        ariaLabel ??
        (lang === "zh"
          ? "One CLI 生成工作区模块的动画，展示前端、后端、文档、共享库、部署和 AI 上下文模块"
          : "One CLI workspace assembly animation showing frontend, backend, docs, library, deploy, and AI context modules")
      }
      className="relative h-[420px] min-w-0 overflow-hidden md:h-[560px] lg:h-[610px]"
      role="img"
    >
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

function mountScene(THREE: Three, canvas: HTMLCanvasElement, container: HTMLElement, lang: Locale) {
  const renderer = new THREE.WebGLRenderer({
    alpha: true,
    antialias: true,
    canvas,
    powerPreference: "high-performance",
  });
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.setClearColor(0x000000, 0);
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 3.5));

  const scene = new THREE.Scene();
  const camera = new THREE.OrthographicCamera(-1, 1, 1, -1, -20, 20);
  camera.up.set(0, 0, 1);
  camera.position.set(3.35, -5.35, 3.85);
  camera.lookAt(0.02, 0, 0.2);

  scene.add(new THREE.AmbientLight(0xffffff, 2.05));

  const keyLight = new THREE.DirectionalLight(0xffffff, 3.7);
  keyLight.position.set(-2.6, -3.2, 5.2);
  scene.add(keyLight);

  const rimLight = new THREE.DirectionalLight(0xfff7ed, 2.9);
  rimLight.position.set(4.4, -1.8, 3.6);
  scene.add(rimLight);

  const amberPulse = new THREE.PointLight(0xea580c, 0.86, 7);
  amberPulse.position.set(0.1, -0.15, 1.6);
  scene.add(amberPulse);

  const grid = createPerspectiveGrid(THREE);
  scene.add(grid);

  const root = new THREE.Group();
  root.position.set(-0.14, 0.02, 0.02);
  root.scale.setScalar(sceneScale);
  scene.add(root);

  const board = createWorkspaceBoard(THREE);
  root.add(board.group);

  const printedMark = createPrintedMark(THREE);
  root.add(printedMark.group);

  const modules = moduleConfigs.map((config, index) => {
    const model = createModule(THREE, config, index, lang);
    root.add(model.route);
    root.add(model.group);
    return model;
  });

  const pointer = {
    x: 0,
    y: 0,
    targetX: 0,
    targetY: 0,
  };
  const motionQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
  const startedAt = performance.now();
  let frame = 0;

  const resize = () => {
    const rect = container.getBoundingClientRect();
    const width = Math.max(320, rect.width);
    const height = Math.max(320, rect.height);
    const aspect = width / height;
    const viewHeight = width < 520 ? 5.04 : 4.84;
    const viewWidth = viewHeight * aspect;

    camera.left = -viewWidth / 2;
    camera.right = viewWidth / 2;
    camera.top = viewHeight / 2;
    camera.bottom = -viewHeight / 2;
    camera.updateProjectionMatrix();
    renderer.setSize(width, height, false);
  };

  const render = (now = performance.now()) => {
    pointer.x += (pointer.targetX - pointer.x) * 0.08;
    pointer.y += (pointer.targetY - pointer.y) * 0.08;

    const elapsed = motionQuery.matches ? settleStart + 900 : Math.max(0, now - startedAt);
    updateScene(THREE, root, board, printedMark, modules, amberPulse, elapsed, pointer.x, pointer.y, motionQuery.matches);
    renderer.render(scene, camera);

    if (!motionQuery.matches) {
      frame = window.requestAnimationFrame(render);
    }
  };

  const onPointerMove = (event: PointerEvent) => {
    const rect = canvas.getBoundingClientRect();
    pointer.targetX = (event.clientX - rect.left) / rect.width - 0.5;
    pointer.targetY = (event.clientY - rect.top) / rect.height - 0.5;
  };

  const onPointerLeave = () => {
    pointer.targetX = 0;
    pointer.targetY = 0;
  };

  const observer = new ResizeObserver(() => {
    resize();
    render();
  });

  observer.observe(container);
  canvas.addEventListener("pointermove", onPointerMove);
  canvas.addEventListener("pointerleave", onPointerLeave);
  resize();
  render();

  return () => {
    observer.disconnect();
    canvas.removeEventListener("pointermove", onPointerMove);
    canvas.removeEventListener("pointerleave", onPointerLeave);
    if (frame) {
      window.cancelAnimationFrame(frame);
    }
    disposeObject(scene);
    renderer.dispose();
  };
}

function createWorkspaceBoard(THREE: Three): BoardModel {
  const geometry = createExtrudedRoundedBox(THREE, boardWidth, boardDepth, boardHeight, 0.24);
  const topMaterial = createSolidMaterial(THREE, {
    color: 0x2a2520,
    emissive: 0x090807,
    roughness: 0.76,
    seed: 11,
  });
  const sideMaterial = createSolidMaterial(THREE, {
    color: 0x15110f,
    emissive: 0x090807,
    roughness: 0.82,
    seed: 17,
  });
  const group = new THREE.Group();
  group.add(new THREE.Mesh(geometry, [topMaterial, sideMaterial]));

  const outline = createOutline(THREE, boardWidth, boardDepth, 0.24, 0xffffff, boardHeight + 0.012);
  setMaxOpacity(outline.material as Material, 0.28);
  group.add(outline);

  const flow = createLiquidFlow(THREE, boardWidth * 0.94, boardDepth * 0.88, 0.18, false);
  flow.position.z = boardHeight + 0.018;
  group.add(flow);

  group.position.z = -0.08;

  return {
    flowMaterial: flow.material as Material,
    flowTexture: ((flow.material as Material & { map?: Texture }).map as Texture),
    group,
    materials: [topMaterial, sideMaterial, outline.material as Material, flow.material as Material],
  };
}

function createPrintedMark(THREE: Three): PrintedMarkModel {
  const group = new THREE.Group();
  group.position.set(launcherX, launcherY, boardTopZ + 0.034);

  const label = createLogoMark(THREE, "one cli");
  group.add(label.group);

  return {
    group,
    labelMaterials: label.materials,
    materials: label.materials,
  };
}

function createModule(THREE: Three, config: ModuleConfig, index: number, lang: Locale): ModuleModel {
  const geometry = createExtrudedRoundedBox(THREE, config.width, config.depth, moduleHeight, 0.1);
  const topMaterial = createSolidMaterial(THREE, {
    color: 0x302b26,
    emissive: 0x070605,
    roughness: 0.78,
    seed: 31 + index * 7,
  });
  const sideMaterial = createSolidMaterial(THREE, {
    color: 0x171412,
    emissive: 0x030303,
    roughness: 0.86,
    seed: 43 + index * 7,
  });
  const group = new THREE.Group();
  group.add(new THREE.Mesh(geometry, [topMaterial, sideMaterial]));

  const outline = createOutline(THREE, config.width, config.depth, 0.1, 0x8a8179, moduleHeight + 0.012);
  setMaxOpacity(outline.material as Material, 0.5);
  group.add(outline);

  createStuds(THREE, config.width, config.depth).forEach((stud) => group.add(stud));

  const labelText = config.labels[lang];
  const labelWidth = lang === "zh" ? 320 : labelText.length > 8 ? 420 : labelText.length > 5 ? 360 : 300;
  const labelFontSize = labelText.length > 8 ? 42 : labelText.length > 5 ? 48 : lang === "zh" ? 62 : 56;
  const label = createWordPlane(
    THREE,
    labelText,
    "rgba(255, 250, 242, 0.98)",
    labelWidth,
    132,
    labelFontSize,
    config.width * 0.9,
  );
  label.position.z = moduleHeight + 0.065;
  group.add(label);

  const start = new THREE.Vector3(launcherX, launcherY, boardTopZ + launcherLift);
  const target = new THREE.Vector3(config.x, config.y, boardTopZ + 0.014);
  const hoverTarget = new THREE.Vector3(config.x, config.y, boardTopZ + 0.42);
  const fromLauncher = new THREE.Vector3(config.x - launcherX, config.y - launcherY, 0).normalize();
  const bend = new THREE.Vector3(-fromLauncher.y, fromLauncher.x, 0).multiplyScalar(index % 2 === 0 ? 0.18 : -0.18);
  const control = new THREE.Vector3(
    launcherX + (config.x - launcherX) * 0.46 + bend.x,
    launcherY + (config.y - launcherY) * 0.46 + bend.y,
    1.1 + (index % 4) * 0.055,
  );
  const route = createRoute(THREE, start, control, hoverTarget);

  group.position.copy(start);
  group.scale.setScalar(0.08);

  return {
    config,
    control,
    group,
    labelMaterials: [label.material as Material],
    hoverTarget,
    materials: [topMaterial, sideMaterial, outline.material as Material, label.material as Material],
    route,
    routeMaterial: route.material as Material,
    start,
    target,
  };
}

function createStuds(THREE: Three, width: number, depth: number) {
  const studs: Mesh[] = [];
  const material = createSolidMaterial(THREE, {
    color: 0x3f3a34,
    emissive: 0x050403,
    roughness: 0.72,
    seed: 71,
  });
  const geometry = new THREE.CylinderGeometry(0.045, 0.05, 0.035, 28);
  geometry.rotateX(Math.PI / 2);

  for (const x of [-width * 0.28, width * 0.28]) {
    const stud = new THREE.Mesh(geometry, material);
    stud.position.set(x, depth * 0.23, moduleHeight + 0.032);
    studs.push(stud);
  }

  return studs;
}

function createRoute(THREE: Three, start: Vector3, control: Vector3, target: Vector3) {
  const points: Vector3[] = [];

  for (let index = 0; index < 42; index += 1) {
    const t = index / 41;
    points.push(quadraticPoint(THREE, start, control, target, t));
  }

  const geometry = new THREE.BufferGeometry().setFromPoints(points);
  geometry.setDrawRange(0, 0);
  const material = new THREE.LineBasicMaterial({
    color: 0xd6d3d1,
    opacity: 0,
    transparent: true,
  });

  return new THREE.Line(geometry, material);
}

function createExtrudedRoundedBox(THREE: Three, width: number, depth: number, height: number, radius: number) {
  const shape = createRoundedRectShape(THREE, width, depth, radius);
  const geometry = new THREE.ExtrudeGeometry(shape, {
    bevelEnabled: true,
    bevelSegments: 3,
    bevelSize: 0.012,
    bevelThickness: 0.012,
    curveSegments: 20,
    depth: height,
    steps: 1,
  });
  geometry.computeVertexNormals();
  return geometry;
}

function createSolidMaterial(
  THREE: Three,
  {
    color,
    emissive,
    roughness,
    seed = 1,
  }: {
    color: number;
    emissive: number;
    roughness: number;
    seed?: number;
  },
) {
  const texture = createSurfaceTexture(THREE, color, seed);
  const material = new THREE.MeshStandardMaterial({
    bumpMap: texture,
    bumpScale: 0.006,
    color: 0xffffff,
    emissive,
    emissiveIntensity: 0.18,
    map: texture,
    metalness: 0.12,
    opacity: 0,
    roughness,
    transparent: true,
  });

  setMaxOpacity(material, 1);
  return material;
}

function createSurfaceTexture(THREE: Three, color: number, seed: number) {
  const canvas = document.createElement("canvas");
  const size = 512;
  canvas.width = size;
  canvas.height = size;

  const ctx = canvas.getContext("2d");
  if (ctx) {
    const base = new THREE.Color(color);
    const r = Math.round(base.r * 255);
    const g = Math.round(base.g * 255);
    const b = Math.round(base.b * 255);

    ctx.fillStyle = `rgb(${r}, ${g}, ${b})`;
    ctx.fillRect(0, 0, size, size);

    const sheen = ctx.createLinearGradient(0, 0, size, size);
    sheen.addColorStop(0, "rgba(255, 255, 255, 0.055)");
    sheen.addColorStop(0.42, "rgba(255, 255, 255, 0)");
    sheen.addColorStop(0.78, "rgba(0, 0, 0, 0.16)");
    ctx.fillStyle = sheen;
    ctx.fillRect(0, 0, size, size);

    for (let index = 0; index < 1600; index += 1) {
      const x = Math.floor(pseudoRandom(seed, index) * size);
      const y = Math.floor(pseudoRandom(seed + 23, index) * size);
      const alpha = 0.015 + pseudoRandom(seed + 41, index) * 0.045;
      const light = pseudoRandom(seed + 59, index) > 0.5 ? 255 : 0;
      ctx.fillStyle = `rgba(${light}, ${light}, ${light}, ${alpha})`;
      ctx.fillRect(x, y, 1, 1);
    }

    ctx.lineWidth = 1;
    for (let index = 0; index < 12; index += 1) {
      const y = pseudoRandom(seed + 83, index) * size;
      const alpha = 0.012 + pseudoRandom(seed + 101, index) * 0.03;
      ctx.strokeStyle = `rgba(255, 255, 255, ${alpha})`;
      ctx.beginPath();
      ctx.moveTo(-20, y);
      ctx.lineTo(size + 20, y + pseudoRandom(seed + 127, index) * 18 - 9);
      ctx.stroke();
    }
  }

  const texture = new THREE.CanvasTexture(canvas);
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.anisotropy = 12;
  texture.magFilter = THREE.LinearFilter;
  texture.minFilter = THREE.LinearMipmapLinearFilter;
  texture.wrapS = THREE.RepeatWrapping;
  texture.wrapT = THREE.RepeatWrapping;
  texture.repeat.set(1.05, 1.05);
  return texture;
}

function pseudoRandom(seed: number, index: number) {
  const value = Math.sin(seed * 97.13 + index * 19.91) * 10000;
  return value - Math.floor(value);
}

function createGlassMaterial(
  THREE: Three,
  {
    color,
    emissive,
    maxOpacity,
    roughness = 0.18,
    transmission = 0.74,
  }: {
    color: number;
    emissive: number;
    maxOpacity: number;
    roughness?: number;
    transmission?: number;
  },
) {
  const material = new THREE.MeshPhysicalMaterial({
    attenuationColor: new THREE.Color(0xfffbf5),
    attenuationDistance: 3.1,
    clearcoat: 1,
    clearcoatRoughness: 0.03,
    color,
    depthWrite: false,
    emissive,
    emissiveIntensity: 0.05,
    envMapIntensity: 2.1,
    ior: 1.46,
    metalness: 0,
    opacity: 0,
    reflectivity: 0.9,
    roughness,
    sheen: 0.42,
    sheenColor: new THREE.Color(0xffffff),
    sheenRoughness: 0.16,
    thickness: 0.82,
    transmission,
    transparent: true,
  });

  setMaxOpacity(material, maxOpacity);
  material.userData.keepTransparent = true;
  return material;
}

function createLiquidFlow(THREE: Three, width: number, depth: number, radius: number, primary: boolean) {
  const geometry = new THREE.ShapeGeometry(createRoundedRectShape(THREE, width, depth, radius));
  const texture = createLiquidFlowTexture(THREE, primary);
  const material = new THREE.MeshBasicMaterial({
    blending: THREE.AdditiveBlending,
    depthWrite: false,
    map: texture,
    opacity: 0,
    transparent: true,
  });

  setMaxOpacity(material, primary ? 0.34 : 0.19);
  return new THREE.Mesh(geometry, material);
}

function createLiquidFlowTexture(THREE: Three, primary: boolean) {
  const canvas = document.createElement("canvas");
  const size = 512;
  canvas.width = size;
  canvas.height = size;

  const ctx = canvas.getContext("2d");
  if (ctx) {
    const glow = ctx.createLinearGradient(0, size, size, 0);
    glow.addColorStop(0, "rgba(255, 255, 255, 0)");
    glow.addColorStop(0.34, primary ? "rgba(255, 255, 255, 0.28)" : "rgba(255, 255, 255, 0.14)");
    glow.addColorStop(0.5, primary ? "rgba(255, 247, 237, 0.24)" : "rgba(255, 247, 237, 0.1)");
    glow.addColorStop(0.56, primary ? "rgba(234, 88, 12, 0.08)" : "rgba(234, 88, 12, 0.035)");
    glow.addColorStop(0.62, "rgba(255, 255, 255, 0)");
    ctx.fillStyle = glow;
    ctx.fillRect(0, 0, size, size);

    for (let index = 0; index < 5; index += 1) {
      const x = -160 + index * 150;
      ctx.strokeStyle = primary ? "rgba(255, 255, 255, 0.24)" : "rgba(255, 255, 255, 0.12)";
      ctx.lineWidth = index % 2 === 0 ? 18 : 8;
      ctx.beginPath();
      ctx.moveTo(x, size + 40);
      ctx.bezierCurveTo(x + 120, size * 0.58, x + 220, size * 0.42, x + 390, -40);
      ctx.stroke();
    }

    ctx.fillStyle = primary ? "rgba(255, 255, 255, 0.16)" : "rgba(255, 255, 255, 0.07)";
    for (let index = 0; index < 28; index += 1) {
      const x = (index * 73) % size;
      const y = (index * 139) % size;
      const r = 1.4 + (index % 4) * 0.8;
      ctx.beginPath();
      ctx.arc(x, y, r, 0, Math.PI * 2);
      ctx.fill();
    }
  }

  const texture = new THREE.CanvasTexture(canvas);
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.wrapS = THREE.RepeatWrapping;
  texture.wrapT = THREE.RepeatWrapping;
  texture.repeat.set(1.15, 1.15);
  return texture;
}

function createOutline(THREE: Three, width: number, depth: number, radius: number, color: number, z = 0) {
  const shape = createRoundedRectShape(THREE, width, depth, radius);
  const points = shape.getPoints(96);
  points.push(points[0].clone());

  const geometry = new THREE.BufferGeometry().setFromPoints(points.map((point) => new THREE.Vector3(point.x, point.y, z)));
  const material = new THREE.LineBasicMaterial({
    color,
    opacity: 0,
    transparent: true,
  });

  return new THREE.Line(geometry, material);
}

function createPerspectiveGrid(THREE: Three) {
  const points: Vector3[] = [];
  const size = 7;
  const step = 0.42;

  for (let value = -size; value <= size; value += step) {
    points.push(new THREE.Vector3(value, -size, -2.6), new THREE.Vector3(value + 2.5, size, -2.6));
    points.push(new THREE.Vector3(-size, value, -2.6), new THREE.Vector3(size, value + 1.8, -2.6));
  }

  const geometry = new THREE.BufferGeometry().setFromPoints(points);
  const material = new THREE.LineBasicMaterial({
    color: 0x78716c,
    opacity: 0.08,
    transparent: true,
  });
  const grid = new THREE.LineSegments(geometry, material);
  grid.position.set(0.35, 0.45, 0);
  return grid;
}

function createLogoMark(THREE: Three, text: string) {
  const group = new THREE.Group();
  const texture = new THREE.TextureLoader().load("/brand/icon.svg", (loadedTexture) => {
    loadedTexture.colorSpace = THREE.SRGBColorSpace;
    loadedTexture.anisotropy = 16;
    loadedTexture.generateMipmaps = false;
    loadedTexture.magFilter = THREE.LinearFilter;
    loadedTexture.minFilter = THREE.LinearFilter;
    loadedTexture.needsUpdate = true;
  });
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.anisotropy = 16;
  texture.generateMipmaps = false;
  texture.magFilter = THREE.LinearFilter;
  texture.minFilter = THREE.LinearFilter;
  const iconMaterial = new THREE.MeshBasicMaterial({
    depthTest: false,
    depthWrite: false,
    map: texture,
    opacity: 0,
    transparent: true,
  });
  iconMaterial.toneMapped = false;
  iconMaterial.userData.keepTransparent = true;
  const icon = new THREE.Mesh(new THREE.PlaneGeometry(0.18, 0.18), iconMaterial);
  icon.position.x = -0.24;
  icon.renderOrder = 10;
  group.add(icon);

  const word = createWordPlane(THREE, text, "rgba(255, 247, 237, 0.95)", 320, 104, 56);
  word.position.x = 0.13;
  group.add(word);

  return {
    group,
    materials: [iconMaterial, word.material as Material],
  };
}

function createWordPlane(
  THREE: Three,
  text: string,
  color: string,
  width: number,
  height: number,
  fontSize: number,
  maxWorldWidth?: number,
) {
  const fontFamily =
    '"Inter", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Noto Sans CJK SC", "Funnel Sans", system-ui, sans-serif';
  const font = `800 ${fontSize}px ${fontFamily}`;
  const measureCanvas = document.createElement("canvas");
  const measureCtx = measureCanvas.getContext("2d");
  let measuredWidth = width;
  let measuredHeight = height;

  if (measureCtx) {
    measureCtx.font = font;
    const metrics = measureCtx.measureText(text);
    const textWidth = Math.ceil(metrics.actualBoundingBoxLeft + metrics.actualBoundingBoxRight || metrics.width);
    const textHeight = Math.ceil(metrics.actualBoundingBoxAscent + metrics.actualBoundingBoxDescent || fontSize);

    measuredWidth = Math.max(width, textWidth + 96);
    measuredHeight = Math.max(height, textHeight + 48);
  }

  const canvas = document.createElement("canvas");
  const scale = 7;
  canvas.width = measuredWidth * scale;
  canvas.height = measuredHeight * scale;

  const ctx = canvas.getContext("2d");
  if (ctx) {
    ctx.scale(scale, scale);
    ctx.imageSmoothingEnabled = true;
    ctx.imageSmoothingQuality = "high";
    ctx.clearRect(0, 0, measuredWidth, measuredHeight);
    ctx.shadowBlur = 0;
    ctx.shadowColor = "transparent";
    ctx.fillStyle = color;
    ctx.font = font;
    ctx.lineJoin = "round";
    ctx.lineWidth = Math.max(0.5, fontSize * 0.012);
    ctx.strokeStyle = "rgba(5, 5, 5, 0.34)";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.strokeText(text, measuredWidth / 2, measuredHeight / 2);
    ctx.fillText(text, measuredWidth / 2, measuredHeight / 2);
  }

  const texture = new THREE.CanvasTexture(canvas);
  texture.anisotropy = 16;
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.generateMipmaps = false;
  texture.magFilter = THREE.LinearFilter;
  texture.minFilter = THREE.LinearFilter;
  const material = new THREE.MeshBasicMaterial({
    depthTest: false,
    depthWrite: false,
    map: texture,
    opacity: 0,
    transparent: true,
  });
  material.toneMapped = false;
  material.userData.keepTransparent = true;
  const worldWidth = maxWorldWidth ?? measuredWidth / 480;
  const worldHeight = worldWidth * (measuredHeight / measuredWidth);
  const mesh = new THREE.Mesh(new THREE.PlaneGeometry(worldWidth, worldHeight), material);
  mesh.renderOrder = 10;
  return mesh;
}

function createRoundedRectShape(THREE: Three, width: number, height: number, radius: number) {
  const shape = new THREE.Shape();
  const x = -width / 2;
  const y = -height / 2;
  const right = width / 2;
  const bottom = height / 2;

  shape.moveTo(x + radius, y);
  shape.lineTo(right - radius, y);
  shape.quadraticCurveTo(right, y, right, y + radius);
  shape.lineTo(right, bottom - radius);
  shape.quadraticCurveTo(right, bottom, right - radius, bottom);
  shape.lineTo(x + radius, bottom);
  shape.quadraticCurveTo(x, bottom, x, bottom - radius);
  shape.lineTo(x, y + radius);
  shape.quadraticCurveTo(x, y, x + radius, y);

  return shape;
}

function updateScene(
  THREE: Three,
  root: Group,
  board: BoardModel,
  printedMark: PrintedMarkModel,
  modules: ModuleModel[],
  amberPulse: import("three").PointLight,
  elapsed: number,
  pointerX: number,
  pointerY: number,
  reducedMotion: boolean,
) {
  const markRaw = reducedMotion ? 1 : clamp01((elapsed - coreStart) / 650);
  const markEnter = easeOutBack(markRaw);
  const settled = reducedMotion ? 1 : smoothStep(settleStart, settleStart + 800, elapsed);
  const sharedBreath = reducedMotion ? 0 : Math.sin(elapsed * 0.00135) * 0.022 * settled;

  root.rotation.x = THREE.MathUtils.degToRad(0.75 + pointerY * 1.5);
  root.rotation.y = THREE.MathUtils.degToRad(pointerX * 1.5);
  root.position.z = 0.02;

  board.group.visible = markRaw > 0.01;
  board.group.scale.setScalar(0.94 + markEnter * 0.06);
  setOpacity(board.materials, (0.35 + markEnter * 0.65) * (1 + sharedBreath));
  setMaterialOpacity(board.flowMaterial, markRaw * (0.12 + Math.sin(elapsed * 0.002) * 0.03));
  board.flowTexture.offset.x = elapsed * 0.00004;
  board.flowTexture.offset.y = -elapsed * 0.00003;

  printedMark.group.visible = markRaw > 0.01;
  printedMark.group.scale.setScalar(0.9 + markEnter * 0.1);
  setOpacity(printedMark.materials, markRaw * 0.94);
  printedMark.labelMaterials.forEach((material) => setMaterialOpacity(material, markRaw * 0.94));
  amberPulse.intensity = 0.54 + Math.sin(elapsed * 0.0032) * 0.18;

  modules.forEach((module, index) => {
    const raw = reducedMotion ? 1 : clamp01((elapsed - moduleStart - module.config.delay) / moduleDuration);
    const lift = smoothStep(0, 0.18, raw);
    const fly = smoothStep(0.14, 0.72, raw);
    const seat = smoothStep(0.72, 1, raw);
    const scaleIn = smoothStep(0.48, 0.88, raw);
    const labelReveal = smoothStep(0.62, 0.92, raw);
    const alpha = smoothStep(0.02, 0.24, raw);
    const point = quadraticPoint(THREE, module.start, module.control, module.hoverTarget, easeInOutCubic(fly));
    const lock = easeOutCubic(seat);
    const moduleSettled = settled * smoothStep(0.82, 1, raw);
    const breath = reducedMotion ? 0 : Math.sin(elapsed * 0.00165 + index * 0.62) * 0.026 * moduleSettled;
    const snapFlash = Math.sin(seat * Math.PI) * smoothStep(0.76, 0.92, raw);
    const scale = 0.2 + scaleIn * 0.8;

    module.group.visible = alpha > 0.01;
    module.group.position.set(
      lerp(point.x, module.target.x, lock),
      lerp(point.y, module.target.y, lock),
      lerp(point.z + lift * 0.16, module.target.z, lock) + breath + snapFlash * 0.035,
    );
    module.group.rotation.x = (index % 2 === 0 ? -0.18 : 0.16) * (1 - smoothStep(0.44, 0.9, raw));
    module.group.rotation.z = (index % 2 === 0 ? -0.46 : 0.46) * (1 - smoothStep(0.34, 0.92, raw));
    module.group.scale.set(scale, scale, (0.42 + scaleIn * 0.58) * (1 + moduleSettled * 0.035));
    setOpacity(module.materials, alpha);
    module.labelMaterials.forEach((material) => setMaterialOpacity(material, alpha * labelReveal * 0.92));

    const routeProgress = smoothStep(0.08, 0.7, raw);
    const routeFade = 1 - smoothStep(0.72, 0.98, raw);
    module.route.geometry.setDrawRange(0, Math.max(0, Math.floor(routeProgress * 42)));
    setMaterialOpacity(module.routeMaterial, alpha * routeFade * 0.26);
  });
}

function quadraticPoint(THREE: Three, start: Vector3, control: Vector3, target: Vector3, progress: number) {
  const t = clamp01(progress);
  const inv = 1 - t;

  return new THREE.Vector3(
    inv * inv * start.x + 2 * inv * t * control.x + t * t * target.x,
    inv * inv * start.y + 2 * inv * t * control.y + t * t * target.y,
    inv * inv * start.z + 2 * inv * t * control.z + t * t * target.z,
  );
}

function setOpacity(materials: Material[], opacity: number) {
  materials.forEach((material) => setMaterialOpacity(material, opacity));
}

function setMaterialOpacity(material: Material, opacity: number) {
  const maxOpacity = typeof material.userData.maxOpacity === "number" ? material.userData.maxOpacity : 1;

  material.opacity = clamp01(opacity * maxOpacity);
  material.transparent = Boolean(material.userData.keepTransparent) || material.opacity < 0.999 || maxOpacity < 0.999;
  material.needsUpdate = true;
}

function setMaxOpacity(material: Material, opacity: number) {
  material.userData.maxOpacity = opacity;
}

function disposeObject(object: Object3D) {
  object.traverse((child) => {
    const mesh = child as Mesh;
    const geometry = mesh.geometry;
    const material = mesh.material;

    geometry?.dispose();

    if (Array.isArray(material)) {
      material.forEach(disposeMaterial);
    } else if (material) {
      disposeMaterial(material);
    }
  });
}

function disposeMaterial(material: Material) {
  const withMap = material as Material & { map?: { dispose: () => void } };
  withMap.map?.dispose();
  material.dispose();
}

function clamp01(value: number) {
  return Math.min(1, Math.max(0, value));
}

function easeInOutCubic(value: number) {
  const clamped = clamp01(value);

  return clamped < 0.5
    ? 4 * clamped * clamped * clamped
    : 1 - Math.pow(-2 * clamped + 2, 3) / 2;
}

function easeOutBack(value: number) {
  const clamped = clamp01(value);
  const c1 = 1.28;
  const c3 = c1 + 1;

  return 1 + c3 * Math.pow(clamped - 1, 3) + c1 * Math.pow(clamped - 1, 2);
}

function easeOutCubic(value: number) {
  const clamped = clamp01(value);

  return 1 - Math.pow(1 - clamped, 3);
}

function smoothStep(from: number, to: number, value: number) {
  const progress = clamp01((value - from) / (to - from));

  return progress * progress * (3 - 2 * progress);
}

function lerp(from: number, to: number, progress: number) {
  return from + (to - from) * progress;
}
