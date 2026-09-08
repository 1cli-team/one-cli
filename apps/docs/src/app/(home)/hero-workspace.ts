import type { Locale } from "@/i18n";

type Three = typeof import("three");

type Group = import("three").Group;
type Line = import("three").Line;
type Material = import("three").Material;
type Mesh = import("three").Mesh;
type Texture = import("three").Texture;
type Vector3 = import("three").Vector3;
type BufferGeometry = import("three").BufferGeometry;

type WorkspaceResources = {
  geometries: Map<string, BufferGeometry>;
  surfaceTextures: Map<string, Texture>;
};

export type ModuleId =
  | "apps"
  | "api"
  | "docs"
  | "packages"
  | "env"
  | "deploy"
  | "manifest"
  | "cli-interface";

type ModuleSpotlightCopy = {
  title: string;
  subtitle: string;
};

type ModuleConfig = {
  id: ModuleId;
  labels: Record<Locale, string>;
  spotlight: Record<Locale, ModuleSpotlightCopy>;
  x: number;
  y: number;
  width: number;
  depth: number;
  delay: number;
};

type ModuleModel = {
  config: ModuleConfig;
  decorativeMaterials: Material[];
  group: Group;
  hitTarget: Mesh;
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
  icon: Mesh;
  labelMaterials: Material[];
  materials: Material[];
};

type BoardModel = {
  group: Group;
  materials: Material[];
  flowMaterial: Material;
  flowTexture: Texture;
};

type ModuleSpotlightModel = {
  config: ModuleConfig;
  group: Group;
  panelGroup: Group;
  materials: Material[];
  signalLines: Line[];
  mix: number;
};

const moduleConfigs: ModuleConfig[] = [
  {
    id: "cli-interface",
    labels: { zh: "CLI接口", en: "CLI Interface" },
    spotlight: {
      zh: { title: "CLI 接口", subtitle: "结构化输出与命令帮助" },
      en: { title: "CLI Interface", subtitle: "JSON output and command help" },
    },
    x: -1.18,
    y: 0.9,
    width: 0.96,
    depth: 0.44,
    delay: 0,
  },
  {
    id: "manifest",
    labels: { zh: "Manifest", en: "Manifest" },
    spotlight: {
      zh: { title: "Manifest", subtitle: "项目清单和工程契约" },
      en: { title: "Manifest", subtitle: "Workspace contract file" },
    },
    x: -0.1,
    y: 0.48,
    width: 0.92,
    depth: 0.44,
    delay: 360,
  },
  {
    id: "deploy",
    labels: { zh: "Deploy", en: "Deploy" },
    spotlight: {
      zh: { title: "Deploy", subtitle: "K8s / S3 / Vercel 部署" },
      en: { title: "Deploy", subtitle: "K8s, S3, Vercel deploy" },
    },
    x: 1.0,
    y: 0.48,
    width: 0.78,
    depth: 0.44,
    delay: 720,
  },
  {
    id: "apps",
    labels: { zh: "Apps", en: "Apps" },
    spotlight: {
      zh: { title: "Apps", subtitle: "Web / Mobile / Desktop 模板" },
      en: { title: "Apps", subtitle: "Web, mobile, desktop" },
    },
    x: -1.18,
    y: 0.2,
    width: 0.78,
    depth: 0.44,
    delay: 1080,
  },
  {
    id: "api",
    labels: { zh: "API", en: "API" },
    spotlight: {
      zh: { title: "API", subtitle: "NestJS / Go 服务" },
      en: { title: "API", subtitle: "NestJS and Go services" },
    },
    x: -0.14,
    y: -0.08,
    width: 0.72,
    depth: 0.44,
    delay: 1440,
  },
  {
    id: "docs",
    labels: { zh: "Docs", en: "Docs" },
    spotlight: {
      zh: { title: "Docs", subtitle: "Starlight 文档站" },
      en: { title: "Docs", subtitle: "Starlight docs site" },
    },
    x: 0.9,
    y: -0.08,
    width: 0.74,
    depth: 0.44,
    delay: 1800,
  },
  {
    id: "packages",
    labels: { zh: "Packages", en: "Packages" },
    spotlight: {
      zh: { title: "Packages", subtitle: "TS / Go 通用库" },
      en: { title: "Packages", subtitle: "TypeScript and Go libs" },
    },
    x: -0.7,
    y: -0.72,
    width: 0.98,
    depth: 0.44,
    delay: 2160,
  },
  {
    id: "env",
    labels: { zh: "Env", en: "Env" },
    spotlight: {
      zh: { title: "Env", subtitle: "dotenv / Infisical 环境" },
      en: { title: "Env", subtitle: "dotenv and Infisical" },
    },
    x: 0.52,
    y: -0.72,
    width: 0.72,
    depth: 0.44,
    delay: 2520,
  },
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
const hitTargetPaddingX = 0.36;
const hitTargetPaddingY = 0.28;

export type WorkspaceModule = ModuleModel;

type WorkspaceUpdate = {
  elapsed: number;
  pointerX: number;
  pointerY: number;
  activeMix: number;
  activeId: ModuleId | null;
  hoveredId: ModuleId | null;
  reducedMotion: boolean;
  focusResponse: number;
};

export function createWorkspace(THREE: Three, lang: Locale, onTextureReady?: () => void) {
  // Share immutable assets within this workspace; each module keeps its own fading materials.
  const resources: WorkspaceResources = { geometries: new Map(), surfaceTextures: new Map() };
  const group = new THREE.Group();
  group.position.set(-0.14, 0.02, 0.02);
  group.scale.setScalar(sceneScale);
  const board = createWorkspaceBoard(THREE, resources);
  const printedMark = createPrintedMark(THREE, resources, onTextureReady);
  const flightPoint = new THREE.Vector3();
  group.add(board.group, printedMark.group);
  const modules = moduleConfigs.map((config, index) => {
    const module = createModule(THREE, config, index, lang, resources);
    group.add(module.route, module.group);
    return module;
  });
  // Reserve the original scene order, but rasterize detail text only when first selected.
  const spotlightGroups = moduleConfigs.map(() => {
    const slot = new THREE.Group();
    slot.visible = false;
    group.add(slot);
    return slot;
  });
  const spotlights: ModuleSpotlightModel[] = [];
  const amberPulse = new THREE.PointLight(0xea580c, 0.86, 7);
  amberPulse.position.set(0.1, -0.15, 1.6);

  const resetFocus = () => {
    for (const spotlight of spotlights) {
      spotlight.mix = 0;
      spotlight.group.visible = false;
      setOpacity(spotlight.materials, 0);
      spotlight.signalLines.forEach((line) => setLineDrawCount(line, 0));
    }
  };

  const update = ({ elapsed, pointerX, pointerY, activeMix, activeId, hoveredId, reducedMotion, focusResponse }: WorkspaceUpdate) => {
    if (activeId && !spotlights.some((spotlight) => spotlight.config.id === activeId)) {
      const index = moduleConfigs.findIndex((config) => config.id === activeId);
      const spotlight = createModuleSpotlight(THREE, moduleConfigs[index], lang, resources, spotlightGroups[index]);
      spotlights.push(spotlight);
    }
    let transitioning = false;
    for (const spotlight of spotlights) {
      const target = activeId === spotlight.config.id ? 1 : 0;
      if (target > 0 || spotlight.mix > 0.001) {
        spotlight.mix += (target - spotlight.mix) * focusResponse;
      } else {
        spotlight.mix = 0;
      }
      transitioning ||= Math.abs(target - spotlight.mix) > 0.001;
    }
    updateScene(THREE, group, board, printedMark, modules, spotlights, amberPulse,
      elapsed, pointerX, pointerY, activeMix, activeId, hoveredId, reducedMotion, flightPoint);
    return transitioning;
  };

  update({ elapsed: -1, pointerX: 0, pointerY: 0, activeMix: 0, activeId: null, hoveredId: null, reducedMotion: false, focusResponse: 1 });
  return {
    group,
    modules,
    update,
    resetFocus,
    launcher: new THREE.Vector3(launcherX - 0.24, launcherY, boardTopZ + 0.04),
    settledAt: settleStart + 800,
    reducedElapsed: settleStart + 900,
    printedMarkIcon: printedMark.icon,
    amberPulse,
  };
}

function createWorkspaceBoard(THREE: Three, resources: WorkspaceResources): BoardModel {
  const geometry = createExtrudedRoundedBox(THREE, boardWidth, boardDepth, boardHeight, 0.24, resources);
  const topMaterial = createSolidMaterial(THREE, {
    color: 0x2a2520,
    emissive: 0x090807,
    roughness: 0.76,
    seed: 11,
  }, resources);
  const sideMaterial = createSolidMaterial(THREE, {
    color: 0x15110f,
    emissive: 0x090807,
    roughness: 0.82,
    seed: 17,
  }, resources);
  const group = new THREE.Group();
  group.add(new THREE.Mesh(geometry, [topMaterial, sideMaterial]));

  const outline = createOutline(THREE, boardWidth, boardDepth, 0.24, 0xffffff, boardHeight + 0.012, resources);
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

function createPrintedMark(THREE: Three, resources: WorkspaceResources, onTextureReady?: () => void): PrintedMarkModel {
  const group = new THREE.Group();
  group.position.set(launcherX, launcherY, boardTopZ + 0.034);
  const label = createLogoMark(THREE, "one cli", resources, onTextureReady);
  group.add(label.group);
  return { group, icon: label.icon, labelMaterials: label.materials, materials: label.materials };
}

function createModule(THREE: Three, config: ModuleConfig, index: number, lang: Locale, resources: WorkspaceResources): ModuleModel {
  const geometry = createExtrudedRoundedBox(THREE, config.width, config.depth, moduleHeight, 0.1, resources);
  const topMaterial = createSolidMaterial(THREE, {
    color: 0x302b26,
    emissive: 0x070605,
    roughness: 0.78,
    seed: 31 + index * 7,
  }, resources);
  const sideMaterial = createSolidMaterial(THREE, {
    color: 0x171412,
    emissive: 0x030303,
    roughness: 0.86,
    seed: 43 + index * 7,
  }, resources);
  const group = new THREE.Group();
  group.add(new THREE.Mesh(geometry, [topMaterial, sideMaterial]));

  const outline = createOutline(THREE, config.width, config.depth, 0.1, 0x8a8179, moduleHeight + 0.012, resources);
  setMaxOpacity(outline.material as Material, 0.5);
  group.add(outline);

  const start = new THREE.Vector3(launcherX, launcherY, boardTopZ + launcherLift);
  const target = new THREE.Vector3(config.x, config.y, boardTopZ + 0.014);
  const hoverTarget = new THREE.Vector3(config.x, config.y, boardTopZ + 0.42);

  const hitTargetMaterial = new THREE.MeshBasicMaterial({
    color: 0xffffff,
    depthWrite: false,
    opacity: 0,
    side: THREE.DoubleSide,
    transparent: true,
  });
  hitTargetMaterial.userData.keepTransparent = true;
  // Material visibility skips rendering while Mesh.raycast still uses its DoubleSide geometry.
  hitTargetMaterial.visible = false;
  const hitTarget = new THREE.Mesh(
    cachedGeometry(resources, `hit:${config.width}:${config.depth}`, () =>
      new THREE.PlaneGeometry(config.width + hitTargetPaddingX, config.depth + hitTargetPaddingY)),
    hitTargetMaterial,
  );
  hitTarget.visible = false;
  hitTarget.position.z = moduleHeight + 0.065;
  group.add(hitTarget);
  hitTarget.renderOrder = 40;

  const studs = createStuds(THREE, config.width, config.depth, resources);
  const decorativeMaterials = uniqueMaterials(studs.map((stud) => stud.material as Material));
  studs.forEach((stud) => group.add(stud));

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
    resources,
  );
  label.position.z = moduleHeight + 0.065;
  group.add(label);

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
    decorativeMaterials,
    group,
    hitTarget,
    labelMaterials: [label.material as Material],
    hoverTarget,
    materials: [topMaterial, sideMaterial, outline.material as Material, label.material as Material],
    route,
    routeMaterial: route.material as Material,
    start,
    target,
  };
}

function createModuleSpotlight(THREE: Three, config: ModuleConfig, lang: Locale, resources: WorkspaceResources, group: Group): ModuleSpotlightModel {
  const panelGroup = new THREE.Group();
  const materials: Material[] = [];
  const signalLines: Line[] = [];
  const copy = config.spotlight[lang];
  const panelOrigin = new THREE.Vector3(config.x + 0.12, config.y + 0.16, boardTopZ + 0.46);
  const panelPosition = getSpotlightPanelPosition(THREE, config);

  group.visible = false;
  panelGroup.position.copy(panelOrigin);
  panelGroup.rotation.x = THREE.MathUtils.degToRad(58);
  panelGroup.userData.originX = panelOrigin.x;
  panelGroup.userData.originY = panelOrigin.y;
  panelGroup.userData.originZ = panelOrigin.z;
  panelGroup.userData.targetX = panelPosition.x;
  panelGroup.userData.targetY = panelPosition.y;
  panelGroup.userData.baseZ = panelPosition.z;
  group.add(panelGroup);

  const panelMaterial = createGlassMaterial(THREE, {
    color: 0x18130f,
    emissive: 0xea580c,
    maxOpacity: 0.8,
    roughness: 0.24,
    transmission: 0.08,
  });
  const panel = new THREE.Mesh(cachedGeometry(resources, "spotlight-panel", () =>
    new THREE.ShapeGeometry(createRoundedRectShape(THREE, 2.08, 0.82, 0.09))), panelMaterial);
  panel.renderOrder = 22;
  panelGroup.add(panel);
  materials.push(panelMaterial);

  const panelOutline = createOutline(THREE, 2.08, 0.82, 0.09, 0xfff7ed, 0.014, resources);
  panelOutline.renderOrder = 24;
  setMaxOpacity(panelOutline.material as Material, 0.66);
  panelGroup.add(panelOutline);
  materials.push(panelOutline.material as Material);

  const title = createWordPlane(
    THREE,
    copy.title,
    "rgba(255, 250, 242, 0.98)",
    420,
    110,
    lang === "zh" ? 70 : 68,
    1.2,
    resources,
  );
  title.position.set(-0.3, 0.12, 0.042);
  title.renderOrder = 32;
  panelGroup.add(title);
  materials.push(title.material as Material);

  const subtitle = createWordPlane(
    THREE,
    copy.subtitle,
    "rgba(231, 229, 228, 0.92)",
    720,
    98,
    lang === "zh" ? 54 : 52,
    1.86,
    resources,
  );
  subtitle.position.set(0, -0.18, 0.044);
  subtitle.renderOrder = 32;
  panelGroup.add(subtitle);
  materials.push(subtitle.material as Material);

  const signalStart = new THREE.Vector3(config.x, config.y, boardTopZ + 0.31);
  const panelSide = config.x < panelPosition.x ? -0.58 : 0.58;
  const stem = createSignalLine(THREE, signalStart, new THREE.Vector3(panelPosition.x + panelSide, panelPosition.y - 0.3, panelPosition.z - 0.04), 0.86, 0xea580c);
  group.add(stem);
  signalLines.push(stem);
  materials.push(stem.material as Material);

  return {
    config,
    group,
    materials,
    mix: 0,
    panelGroup,
    signalLines,
  };
}

function getSpotlightPanelPosition(THREE: Three, config: ModuleConfig) {
  return new THREE.Vector3(THREE.MathUtils.clamp(config.x * 0.42, -0.62, 0.62), 1.62, 0.9);
}

function createSignalLine(THREE: Three, start: Vector3, target: Vector3, lift: number, color: number) {
  const control = new THREE.Vector3((start.x + target.x) / 2, (start.y + target.y) / 2, lift);
  const points: Vector3[] = [];

  for (let index = 0; index < 36; index += 1) {
    points.push(quadraticPoint(THREE, start, control, target, index / 35));
  }

  const geometry = new THREE.BufferGeometry().setFromPoints(points);
  geometry.setDrawRange(0, 0);

  const material = new THREE.LineBasicMaterial({
    blending: THREE.AdditiveBlending,
    color,
    depthWrite: false,
    opacity: 0,
    transparent: true,
  });
  material.userData.keepTransparent = true;
  setMaxOpacity(material, color === 0xea580c ? 0.34 : 0.62);

  const line = new THREE.Line(geometry, material);
  line.renderOrder = 25;
  return line;
}

function createStuds(THREE: Three, width: number, depth: number, resources: WorkspaceResources) {
  const studs: Mesh[] = [];
  const material = createSolidMaterial(THREE, {
    color: 0x3f3a34,
    emissive: 0x050403,
    roughness: 0.72,
    seed: 71,
  }, resources);
  setMaxOpacity(material, 0.42);
  const geometry = cachedGeometry(resources, "stud", () => {
    const cylinder = new THREE.CylinderGeometry(0.045, 0.05, 0.035, 28);
    cylinder.rotateX(Math.PI / 2);
    return cylinder;
  });

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

function cachedGeometry(resources: WorkspaceResources, key: string, create: () => BufferGeometry) {
  let geometry = resources.geometries.get(key);
  if (!geometry) {
    geometry = create();
    resources.geometries.set(key, geometry);
  }
  return geometry;
}

function createExtrudedRoundedBox(THREE: Three, width: number, depth: number, height: number, radius: number, resources: WorkspaceResources) {
  return cachedGeometry(resources, `box:${width}:${depth}:${height}:${radius}`, () => {
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
  });
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
  resources: WorkspaceResources,
) {
  const texture = createSurfaceTexture(THREE, color, seed, resources);
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

function createSurfaceTexture(THREE: Three, color: number, seed: number, resources: WorkspaceResources) {
  const key = `${color}:${seed}`;
  const cached = resources.surfaceTextures.get(key);
  if (cached) return cached;
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
  resources.surfaceTextures.set(key, texture);
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

function createOutline(THREE: Three, width: number, depth: number, radius: number, color: number, z: number, resources: WorkspaceResources) {
  const geometry = cachedGeometry(resources, `outline:${width}:${depth}:${radius}:${z}`, () => {
    const shape = createRoundedRectShape(THREE, width, depth, radius);
    const points = shape.getPoints(96);
    points.push(points[0].clone());
    return new THREE.BufferGeometry().setFromPoints(points.map((point) => new THREE.Vector3(point.x, point.y, z)));
  });
  const material = new THREE.LineBasicMaterial({
    color,
    opacity: 0,
    transparent: true,
  });

  return new THREE.Line(geometry, material);
}

function createLogoMark(THREE: Three, text: string, resources: WorkspaceResources, onTextureReady?: () => void) {
  const group = new THREE.Group();
  const texture = new THREE.TextureLoader().load("/brand/icon.svg", (loadedTexture) => {
    loadedTexture.colorSpace = THREE.SRGBColorSpace;
    loadedTexture.anisotropy = 16;
    loadedTexture.generateMipmaps = false;
    loadedTexture.magFilter = THREE.LinearFilter;
    loadedTexture.minFilter = THREE.LinearFilter;
    loadedTexture.needsUpdate = true;
    onTextureReady?.();
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
  const icon = new THREE.Mesh(cachedGeometry(resources, "plane:0.18:0.18", () =>
    new THREE.PlaneGeometry(0.18, 0.18)), iconMaterial);
  icon.position.x = -0.24;
  icon.renderOrder = 10;
  group.add(icon);

  const word = createWordPlane(THREE, text, "rgba(255, 247, 237, 0.95)", 320, 104, 56, undefined, resources);
  word.position.x = 0.13;
  group.add(word);

  return {
    group,
    icon,
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
  maxWorldWidth: number | undefined,
  resources: WorkspaceResources,
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
  const geometry = cachedGeometry(resources, `plane:${worldWidth}:${worldHeight}`, () =>
    new THREE.PlaneGeometry(worldWidth, worldHeight));
  const mesh = new THREE.Mesh(geometry, material);
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
  moduleSpotlights: ModuleSpotlightModel[],
  amberPulse: import("three").PointLight,
  elapsed: number,
  pointerX: number,
  pointerY: number,
  activeMix: number,
  activeId: ModuleId | null,
  hoveredId: ModuleId | null,
  reducedMotion: boolean,
  flightPoint: Vector3,
) {
  const markRaw = reducedMotion ? 1 : clamp01((elapsed - coreStart) / 650);
  const markEnter = easeOutBack(markRaw);
  const settled = reducedMotion ? 1 : smoothStep(settleStart, settleStart + 800, elapsed);
  const sharedBreath = reducedMotion ? 0 : Math.sin(elapsed * 0.00135) * 0.022 * settled;
  const boardTurnEase = easeInOutCubic(smoothStep(0, 1, activeMix));
  const focusEase = easeInOutCubic(smoothStep(0.08, 1, activeMix));

  root.rotation.x = THREE.MathUtils.degToRad(lerp(0.75 + pointerY * 1.5, -0.7, boardTurnEase));
  root.rotation.y = THREE.MathUtils.degToRad(lerp(pointerX * 1.5, 0, boardTurnEase));
  root.rotation.z = THREE.MathUtils.degToRad(lerp(0, -1.25, boardTurnEase));
  root.position.set(-0.14, lerp(0.02, -0.01, boardTurnEase), 0.02 + boardTurnEase * 0.025);

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
    const isHovered = hoveredId === module.config.id;
    const selected = activeId === module.config.id ? focusEase : 0;
    const raw = reducedMotion ? 1 : clamp01((elapsed - moduleStart - module.config.delay) / moduleDuration);
    const lift = smoothStep(0, 0.18, raw);
    const fly = smoothStep(0.14, 0.72, raw);
    const seat = smoothStep(0.72, 1, raw);
    const scaleIn = smoothStep(0.48, 0.88, raw);
    const labelReveal = smoothStep(0.62, 0.92, raw);
    const alpha = smoothStep(0.02, 0.24, raw);
    const point = quadraticPoint(THREE, module.start, module.control, module.hoverTarget, easeInOutCubic(fly), flightPoint);
    const lock = easeOutCubic(seat);
    const moduleSettled = settled * smoothStep(0.82, 1, raw);
    const breath = reducedMotion ? 0 : Math.sin(elapsed * 0.00165 + index * 0.62) * 0.026 * moduleSettled;
    const snapFlash = Math.sin(seat * Math.PI) * smoothStep(0.76, 0.92, raw);
    const hoverAmount = isHovered ? smoothStep(0.18, 1, moduleSettled) * (1 - selected) : 0;
    const hoverLift = reducedMotion ? 0 : hoverAmount * 0.1;
    const selectedLift = selected * 0.18;
    const selectedScale = selected * 0.045;
    const hoverScale = hoverAmount * 0.045;
    const scale = 0.2 + scaleIn * 0.8;
    const surfaceOpacity = alpha;
    const labelOpacity = alpha * labelReveal * (0.92 + selected * 0.08 + hoverAmount * 0.08);
    const baseX = lerp(point.x, module.target.x, lock);
    const baseY = lerp(point.y, module.target.y, lock);
    const baseZ = lerp(point.z + lift * 0.16, module.target.z, lock) + breath + snapFlash * 0.035;

    module.group.visible = alpha > 0.01;
    module.group.position.set(
      baseX,
      baseY,
      baseZ + hoverLift + selectedLift,
    );
    module.group.rotation.x = (index % 2 === 0 ? -0.18 : 0.16) * (1 - smoothStep(0.44, 0.9, raw));
    module.group.rotation.z = (index % 2 === 0 ? -0.46 : 0.46) * (1 - smoothStep(0.34, 0.92, raw));
    module.group.scale.set(
      scale * (1 + selectedScale + hoverScale),
      scale * (1 + selectedScale + hoverScale),
      (0.42 + scaleIn * 0.58) * (1 + moduleSettled * 0.035 + selected * 0.08),
    );
    module.hitTarget.visible = alpha > 0.65;
    setOpacity(module.materials, surfaceOpacity * (1 + selected * 0.08 + hoverAmount * 0.06));
    setOpacity(module.decorativeMaterials, surfaceOpacity * lerp(0.72, 0.32, selected));
    module.labelMaterials.forEach((material) => setMaterialOpacity(material, labelOpacity));

    const routeProgress = smoothStep(0.08, 0.7, raw);
    const routeFade = 1 - smoothStep(0.72, 0.98, raw);
    setLineDrawCount(module.route, Math.max(0, Math.floor(routeProgress * 42)));
    setMaterialOpacity(module.routeMaterial, alpha * routeFade * 0.26);
  });

  updateModuleSpotlights(moduleSpotlights, elapsed, reducedMotion);
}

function updateModuleSpotlights(
  spotlights: ModuleSpotlightModel[],
  elapsed: number,
  reducedMotion: boolean,
) {
  spotlights.forEach((spotlight) => {
    if (spotlight.mix <= 0.001) {
      if (spotlight.group.visible) {
        setOpacity(spotlight.materials, 0);
        spotlight.signalLines.forEach((line) => setLineDrawCount(line, 0));
        spotlight.group.visible = false;
      }

      return;
    }

    const reveal = easeOutCubic(smoothStep(0.18, 1, spotlight.mix));
    const lineReveal = smoothStep(0.24, 0.9, spotlight.mix);
    const panelReveal = smoothStep(0.34, 1, spotlight.mix);
    const panelMotion = easeInOutCubic(smoothStep(0.24, 1, spotlight.mix));
    const originX = typeof spotlight.panelGroup.userData.originX === "number" ? spotlight.panelGroup.userData.originX : -1.12;
    const originY = typeof spotlight.panelGroup.userData.originY === "number" ? spotlight.panelGroup.userData.originY : 1.1;
    const originZ = typeof spotlight.panelGroup.userData.originZ === "number" ? spotlight.panelGroup.userData.originZ : 0.56;
    const targetX = typeof spotlight.panelGroup.userData.targetX === "number" ? spotlight.panelGroup.userData.targetX : -0.08;
    const targetY = typeof spotlight.panelGroup.userData.targetY === "number" ? spotlight.panelGroup.userData.targetY : 1.18;
    const baseZ = typeof spotlight.panelGroup.userData.baseZ === "number" ? spotlight.panelGroup.userData.baseZ : 0.92;
    const pulse = reducedMotion ? 0 : Math.sin(elapsed * 0.004) * 0.018;

    spotlight.group.visible = spotlight.mix > 0.01;
    spotlight.panelGroup.position.set(
      lerp(originX, targetX, panelMotion),
      lerp(originY, targetY, panelMotion),
      lerp(originZ, baseZ + panelReveal * 0.16, panelMotion) + pulse * panelReveal,
    );
    spotlight.panelGroup.scale.setScalar(0.62 + panelMotion * 0.38);
    setOpacity(spotlight.materials, reveal);

    spotlight.signalLines.forEach((line) => {
      setLineDrawCount(line, Math.max(0, Math.floor(lineReveal * 36)));
    });
  });
}

function quadraticPoint(THREE: Three, start: Vector3, control: Vector3, target: Vector3, progress: number, output = new THREE.Vector3()) {
  const t = clamp01(progress);
  const inv = 1 - t;

  return output.set(
    inv * inv * start.x + 2 * inv * t * control.x + t * t * target.x,
    inv * inv * start.y + 2 * inv * t * control.y + t * t * target.y,
    inv * inv * start.z + 2 * inv * t * control.z + t * t * target.z,
  );
}

function setOpacity(materials: Material[], opacity: number) {
  materials.forEach((material) => setMaterialOpacity(material, opacity));
}

function setLineDrawCount(line: Line, count: number) {
  if (line.userData.drawCount === count) {
    return;
  }

  line.geometry.setDrawRange(0, count);
  line.userData.drawCount = count;
}

function uniqueMaterials(materials: Material[]) {
  return Array.from(new Set(materials));
}

function setMaterialOpacity(material: Material, opacity: number) {
  const maxOpacity = typeof material.userData.maxOpacity === "number" ? material.userData.maxOpacity : 1;
  const visibleOpacity = clamp01(opacity * maxOpacity);
  const nextTransparent = Boolean(material.userData.keepTransparent) || visibleOpacity < 0.999 || maxOpacity < 0.999;
  const nextDepthWrite = !nextTransparent;

  if (Math.abs(material.opacity - visibleOpacity) > 0.001) {
    material.opacity = visibleOpacity;
  }

  // Preserve the original opacity tolerance: tiny nonzero residuals must still render.
  material.visible = material.opacity !== 0;

  if (material.transparent !== nextTransparent) {
    material.transparent = nextTransparent;
    material.needsUpdate = true;
  }

  if (material.depthWrite !== nextDepthWrite) {
    material.depthWrite = nextDepthWrite;
    material.needsUpdate = true;
  }
}

function setMaxOpacity(material: Material, opacity: number) {
  material.userData.maxOpacity = opacity;
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
