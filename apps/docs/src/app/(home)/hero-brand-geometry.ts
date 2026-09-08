export function createBrandArcGeometry(THREE: typeof import("three")): import("three").BufferGeometry {
  const radius = 1.24;
  const halfWidth = radius * 0.45 / 2;
  const sweep = THREE.MathUtils.degToRad(130);
  const thickness = 0.18;
  const bevelSize = 0.035;
  const bevelThickness = 0.035;
  const baseHalfWidth = halfWidth - bevelSize;
  const depth = thickness - bevelThickness * 2;
  const endX = Math.cos(sweep) * radius;
  const endY = Math.sin(sweep) * radius;

  // A circular stroke with two semicircular caps, kept about the SVG's origin.
  // Extrusion adds the bevel outside this path, so inset the stroke by its size.
  const shape = new THREE.Shape();
  shape.moveTo(radius + baseHalfWidth, 0);
  shape.absarc(0, 0, radius + baseHalfWidth, 0, sweep, false);
  shape.absarc(endX, endY, baseHalfWidth, sweep, sweep + Math.PI, false);
  shape.absarc(0, 0, radius - baseHalfWidth, sweep, 0, true);
  shape.absarc(radius, 0, baseHalfWidth, Math.PI, Math.PI * 2, false);
  shape.closePath();

  const geometry = new THREE.ExtrudeGeometry(shape, {
    depth,
    steps: 1,
    bevelEnabled: true,
    bevelSize,
    bevelThickness,
    bevelSegments: 4,
    curveSegments: 48,
  });
  // Preserve the circular centre at XY = (0, 0); only centre the thickness.
  geometry.translate(0, 0, -depth / 2);

  // ExtrudeGeometry duplicates vertices between triangles. Give those copies
  // identical analytic normals so the rounded stroke has smooth metal highlights.
  const positions = geometry.getAttribute("position");
  const normals = geometry.getAttribute("normal");
  const normal = new THREE.Vector3();
  for (let index = 0; index < positions.count; index += 1) {
    const x = positions.getX(index);
    const y = positions.getY(index);
    const z = positions.getZ(index);
    const angle = THREE.MathUtils.clamp(Math.atan2(y, x), 0, sweep);
    const deltaX = x - Math.cos(angle) * radius;
    const deltaY = y - Math.sin(angle) * radius;
    const distance = Math.hypot(deltaX, deltaY);
    const verticalBevel = Math.max(0, Math.abs(z) - depth / 2);

    if (verticalBevel < 1e-7) {
      normal.set(deltaX / distance, deltaY / distance, 0);
    } else {
      const radialNormal = Math.max(0, distance - baseHalfWidth) / (bevelSize * bevelSize);
      const verticalNormal = Math.sign(z) * verticalBevel / (bevelThickness * bevelThickness);
      normal.set(deltaX / distance * radialNormal, deltaY / distance * radialNormal, verticalNormal).normalize();
    }
    normals.setXYZ(index, normal.x, normal.y, normal.z);
  }
  normals.needsUpdate = true;
  geometry.computeBoundingBox();
  geometry.computeBoundingSphere();
  return geometry;
}
