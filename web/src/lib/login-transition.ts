import * as THREE from 'three'

export type LoginTransitionStage =
  | 'idle'
  | 'authenticating'
  | 'orbital-login'
  | 'node-reveal'
  | 'node-fall'
  | 'page-enter'
  | 'complete'
  | 'fallback'

type SceneOptions = {
  onStage?: (stage: LoginTransitionStage) => void
  onComplete?: (fallback: boolean) => void
}

const NODE_REVEAL_MS = 1_150
const NODE_FALL_MS = 1_650
const PAGE_ENTER_MS = 720
const TAU = Math.PI * 2

function clamp(value: number, min = 0, max = 1) {
  return Math.max(min, Math.min(max, value))
}

function easeOutCubic(value: number) {
  const t = clamp(value)
  return 1 - Math.pow(1 - t, 3)
}

function easeInOutCubic(value: number) {
  const t = clamp(value)
  return t < .5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2
}

function seeded(seed: number) {
  const value = Math.sin(seed * 12.9898) * 43758.5453
  return value - Math.floor(value)
}

function canvasTexture(width: number, height: number, draw: (context: CanvasRenderingContext2D) => void) {
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const context = canvas.getContext('2d')
  if (!context) throw new Error('Canvas texture context is unavailable')
  draw(context)
  const texture = new THREE.CanvasTexture(canvas)
  texture.colorSpace = THREE.SRGBColorSpace
  texture.wrapS = THREE.RepeatWrapping
  texture.anisotropy = 4
  return texture
}

function terrainNoise(x: number, y: number) {
  const lattice = (ix: number, iy: number) => {
    const value = Math.sin(ix * 127.1 + iy * 311.7 + 19.19) * 43758.5453
    return value - Math.floor(value)
  }
  const smooth = (value: number) => value * value * (3 - 2 * value)
  const sample = (scale: number) => {
    const px = x * scale
    const py = y * scale
    const x0 = Math.floor(px)
    const y0 = Math.floor(py)
    const fx = smooth(px - x0)
    const fy = smooth(py - y0)
    const a = lattice(x0, y0)
    const b = lattice(x0 + 1, y0)
    const c = lattice(x0, y0 + 1)
    const d = lattice(x0 + 1, y0 + 1)
    return (a + (b - a) * fx) * (1 - fy) + (c + (d - c) * fx) * fy
  }
  return sample(2.3) * .5 + sample(5.7) * .3 + sample(13.4) * .14 + sample(31.8) * .06
}

function createEarthTexture(quality: number) {
  return canvasTexture(quality, quality / 2, context => {
    const width = context.canvas.width
    const height = context.canvas.height
    const image = context.createImageData(width, height)
    const pixels = image.data
    for (let y = 0; y < height; y += 1) {
      const latitude = y / (height - 1)
      const polar = Math.abs(latitude * 2 - 1)
      for (let x = 0; x < width; x += 1) {
        const longitude = x / width
        const warpedX = longitude * 2.2 + Math.sin(latitude * 17) * .045
        const warpedY = latitude * 1.65 + Math.sin(longitude * 13) * .035
        const terrain = terrainNoise(warpedX, warpedY)
        const coast = terrain + Math.sin(longitude * 31 + latitude * 9) * .035
        const land = clamp((coast - .535) * 8.5)
        const elevation = clamp((coast - .575) * 3.7)
        const ice = clamp((polar - .72) * 5.4)
        const shade = .72 + terrain * .25
        const index = (y * width + x) * 4
        const oceanR = 5 + shade * 12
        const oceanG = 24 + shade * 34
        const oceanB = 39 + shade * 48
        const landR = 31 + elevation * 48 + terrain * 18
        const landG = 72 + elevation * 58 + terrain * 28
        const landB = 61 + elevation * 32 + terrain * 22
        pixels[index] = Math.round(oceanR * (1 - land) + landR * land + ice * 178)
        pixels[index + 1] = Math.round(oceanG * (1 - land) + landG * land + ice * 170)
        pixels[index + 2] = Math.round(oceanB * (1 - land) + landB * land + ice * 160)
        pixels[index + 3] = 255
      }
    }
    context.putImageData(image, 0, 0)

    context.globalCompositeOperation = 'screen'
    const polarLight = context.createLinearGradient(0, 0, 0, height)
    polarLight.addColorStop(0, 'rgba(228, 249, 243, .32)')
    polarLight.addColorStop(.1, 'rgba(228, 249, 243, 0)')
    polarLight.addColorStop(.9, 'rgba(228, 249, 243, 0)')
    polarLight.addColorStop(1, 'rgba(228, 249, 243, .28)')
    context.fillStyle = polarLight
    context.fillRect(0, 0, width, height)
  })
}

function createEarthBumpTexture(quality: number) {
  return canvasTexture(quality, quality / 2, context => {
    const width = context.canvas.width
    const height = context.canvas.height
    const image = context.createImageData(width, height)
    const pixels = image.data
    for (let y = 0; y < height; y += 1) {
      for (let x = 0; x < width; x += 1) {
        const value = Math.round(38 + terrainNoise(x / width * 2.2, y / height * 1.65) * 145)
        const index = (y * width + x) * 4
        pixels[index] = value
        pixels[index + 1] = value
        pixels[index + 2] = value
        pixels[index + 3] = 255
      }
    }
    context.putImageData(image, 0, 0)
  })
}

function createCloudTexture(quality: number) {
  return canvasTexture(quality, quality / 2, context => {
    const width = context.canvas.width
    const height = context.canvas.height
    context.clearRect(0, 0, width, height)
    context.filter = `blur(${Math.max(4, quality / 210)}px)`
    // Broad cloud banks keep their silhouette during the node close-up.
    for (let index = 0; index < 72; index += 1) {
      const x = seeded(index + 310) * width
      const y = height * (.16 + seeded(index + 390) * .68)
      const radiusX = width * (.018 + seeded(index + 470) * .065)
      const radiusY = height * (.014 + seeded(index + 520) * .045)
      const alpha = .24 + seeded(index + 610) * .42
      context.fillStyle = `rgba(236, 249, 244, ${alpha})`
      context.beginPath()
      context.ellipse(x, y, radiusX, radiusY, seeded(index + 690) * Math.PI, 0, TAU)
      context.fill()
    }
    // Smaller wisps add parallax detail to the rotating outer shell.
    for (let index = 0; index < 110; index += 1) {
      const x = seeded(index + 730) * width
      const y = seeded(index + 810) * height
      const radiusX = width * (.006 + seeded(index + 870) * .026)
      const radiusY = height * (.004 + seeded(index + 920) * .016)
      context.fillStyle = `rgba(214, 242, 234, ${.12 + seeded(index + 980) * .28})`
      context.beginPath()
      context.ellipse(x, y, radiusX, radiusY, seeded(index + 1030) * Math.PI, 0, TAU)
      context.fill()
    }
    context.filter = 'none'
  })
}

function createLogoTexture(size = 512) {
  return canvasTexture(size, size, context => {
    context.clearRect(0, 0, size, size)
    context.strokeStyle = '#d7f3ed'
    context.fillStyle = '#d7f3ed'
    context.lineWidth = size * .025
    context.beginPath()
    context.arc(size / 2, size / 2, size * .29, .18 * Math.PI, 1.82 * Math.PI)
    context.stroke()
    context.fillRect(size * .47, size * .26, size * .055, size * .48)
    context.fillRect(size * .29, size * .47, size * .42, size * .055)
    context.font = `600 ${size * .09}px monospace`
    context.textAlign = 'center'
    context.fillText('CYBERLIFE', size / 2, size * .9)
  })
}

function createChipFaceTexture(size = 1024) {
  return canvasTexture(size, size, context => {
    context.fillStyle = '#050607'
    context.fillRect(0, 0, size, size)
    context.strokeStyle = '#f2f3ef'
    context.lineWidth = size * .018
    context.strokeRect(size * .06, size * .06, size * .88, size * .88)
    context.strokeStyle = '#aeb2ad'
    context.lineWidth = size * .008
    context.strokeRect(size * .11, size * .11, size * .78, size * .78)

    context.strokeStyle = '#f6f7f3'
    context.lineWidth = size * .012
    const routes = [
      [[.16, .22], [.34, .22], [.44, .32], [.84, .32]],
      [[.16, .5], [.28, .5], [.38, .4], [.84, .4]],
      [[.16, .78], [.35, .78], [.46, .68], [.84, .68]],
      [[.22, .16], [.22, .34], [.32, .44], [.32, .84]],
      [[.5, .16], [.5, .3], [.6, .4], [.6, .84]],
      [[.78, .16], [.78, .34], [.68, .44], [.68, .84]],
    ]
    for (const route of routes) {
      context.beginPath()
      route.forEach(([x, y], index) => index === 0 ? context.moveTo(x * size, y * size) : context.lineTo(x * size, y * size))
      context.stroke()
    }

    context.fillStyle = '#f1f2ee'
    for (const [x, y] of [[.16, .22], [.84, .32], [.16, .5], [.84, .4], [.16, .78], [.84, .68], [.22, .16], [.32, .84], [.5, .16], [.6, .84], [.78, .16], [.68, .84]]) {
      context.beginPath()
      context.arc(x * size, y * size, size * .028, 0, TAU)
      context.fill()
    }

    context.strokeStyle = '#ffffff'
    context.lineWidth = size * .022
    context.beginPath()
    context.arc(size / 2, size / 2, size * .14, 0, TAU)
    context.stroke()
    context.fillStyle = '#ffffff'
    context.beginPath()
    context.arc(size / 2, size / 2, size * .045, 0, TAU)
    context.fill()
    context.font = `700 ${size * .065}px monospace`
    context.textAlign = 'center'
    context.fillText('CL // NODE 01', size / 2, size * .94)
  })
}

function createOrbit(radius: number, color: number, opacity: number) {
  const points = Array.from({ length: 129 }, (_, index) => {
    const angle = index / 128 * TAU
    return new THREE.Vector3(Math.cos(angle) * radius, 0, Math.sin(angle) * radius)
  })
  return new THREE.Line(
    new THREE.BufferGeometry().setFromPoints(points),
    new THREE.LineBasicMaterial({ color, transparent: true, opacity, depthWrite: false }),
  )
}

function createChamferedPlate(width: number, chamfer: number, depth: number, material: THREE.Material) {
  const half = width / 2
  const shape = new THREE.Shape()
  shape.moveTo(-half + chamfer, -half)
  shape.lineTo(half - chamfer, -half)
  shape.lineTo(half, -half + chamfer)
  shape.lineTo(half, half - chamfer)
  shape.lineTo(half - chamfer, half)
  shape.lineTo(-half + chamfer, half)
  shape.lineTo(-half, half - chamfer)
  shape.lineTo(-half, -half + chamfer)
  shape.closePath()
  const geometry = new THREE.ExtrudeGeometry(shape, { depth, bevelEnabled: true, bevelSegments: 2, bevelSize: .055, bevelThickness: .055 })
  geometry.center()
  return new THREE.Mesh(geometry, material)
}

function createChipLine(points: Array<[number, number]>, z: number, material: THREE.Material) {
  return new THREE.Line(
    new THREE.BufferGeometry().setFromPoints(points.map(([x, y]) => new THREE.Vector3(x, y, z))),
    material,
  )
}

function disposeObject(root: THREE.Object3D) {
  const textures = new Set<THREE.Texture>()
  root.traverse(object => {
    const mesh = object as THREE.Mesh
    mesh.geometry?.dispose()
    const materials = mesh.material ? (Array.isArray(mesh.material) ? mesh.material : [mesh.material]) : []
    for (const material of materials) {
      for (const value of Object.values(material)) {
        if (value instanceof THREE.Texture) textures.add(value)
      }
      material.dispose()
    }
  })
  textures.forEach(texture => texture.dispose())
}

/** CyberLife WebGL scene with original geometry and an attributed public-domain Earth texture. */
export function createLoginTransition(canvas: HTMLCanvasElement, options: SceneOptions = {}) {
  let renderer: THREE.WebGLRenderer
  try {
    renderer = new THREE.WebGLRenderer({ canvas, alpha: false, antialias: window.innerWidth > 720, powerPreference: 'high-performance' })
  } catch (error) {
    throw new Error(`WebGL initialization failed: ${error instanceof Error ? error.message : String(error)}`)
  }

  const mobile = window.innerWidth < 720
  const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
  const scene = new THREE.Scene()
  scene.background = new THREE.Color(0x02060b)
  scene.fog = new THREE.FogExp2(0x02060b, .024)
  const camera = new THREE.PerspectiveCamera(42, 1, .1, 100)
  camera.position.set(0, .35, 10)

  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, mobile ? 1.25 : 1.75))
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 1.05

  const ambient = new THREE.HemisphereLight(0xa7e6df, 0x020711, 1.2)
  const keyLight = new THREE.DirectionalLight(0xd8fff8, 3.2)
  keyLight.position.set(-5, 5, 7)
  const rimLight = new THREE.PointLight(0x9fc7d2, 12, 24, 2)
  rimLight.position.set(5, 1, 3)
  const warmLight = new THREE.PointLight(0xe8bd82, 18, 18, 2)
  warmLight.position.set(-4, -3, 4)
  scene.add(ambient, keyLight, rimLight, warmLight)

  const starCount = mobile ? 650 : 1_450
  const starPositions = new Float32Array(starCount * 3)
  const starColors = new Float32Array(starCount * 3)
  for (let index = 0; index < starCount; index += 1) {
    const radius = 17 + seeded(index + 30) * 36
    const theta = seeded(index + 100) * TAU
    const phi = Math.acos(2 * seeded(index + 220) - 1)
    starPositions[index * 3] = radius * Math.sin(phi) * Math.cos(theta)
    starPositions[index * 3 + 1] = radius * Math.cos(phi)
    starPositions[index * 3 + 2] = radius * Math.sin(phi) * Math.sin(theta)
    const tint = .72 + seeded(index + 360) * .28
    starColors[index * 3] = tint * .76
    starColors[index * 3 + 1] = tint * .92
    starColors[index * 3 + 2] = tint
  }
  const starGeometry = new THREE.BufferGeometry()
  starGeometry.setAttribute('position', new THREE.BufferAttribute(starPositions, 3))
  starGeometry.setAttribute('color', new THREE.BufferAttribute(starColors, 3))
  const stars = new THREE.Points(starGeometry, new THREE.PointsMaterial({ size: mobile ? .035 : .045, vertexColors: true, transparent: true, opacity: .82, sizeAttenuation: true }))
  scene.add(stars)

  // Keep the procedural fallback light; the high-resolution public-domain map
  // below replaces it as soon as the browser finishes loading the asset.
  const textureQuality = mobile ? 512 : 768
  const earthTexture = createEarthTexture(textureQuality)
  const earthBumpTexture = createEarthBumpTexture(mobile ? 256 : 384)
  const surfaceCloudFallback = createCloudTexture(textureQuality)
  const backdropCloudTexture = createCloudTexture(textureQuality)
  const sphereSegments = mobile ? 48 : 72

  // The opening shot uses a deliberately oversized Earth: only its right and
  // lower arc enters frame, leaving negative space for the orbital subject.
  // Fourfold scale keeps the planet as a cropped foreground arc, not a small globe.
  // Keep the oversized planet behind the camera while its right and lower arc
  // remains prominent in the first frame.
  const earthRadius = mobile ? 14.5 : 18.5
  const earthGroup = new THREE.Group()
  earthGroup.position.set(mobile ? 7.2 : 10.5, mobile ? -9.2 : -9.5, -16.5)
  earthGroup.rotation.z = -.16
  scene.add(earthGroup)

  const earth = new THREE.Mesh(
    new THREE.SphereGeometry(earthRadius, sphereSegments, sphereSegments),
    new THREE.MeshStandardMaterial({ map: earthTexture, bumpMap: earthBumpTexture, bumpScale: .12, roughness: .82, metalness: .04, color: 0x83b4b1, transparent: true }),
  )
  earth.rotation.z = -.22
  earthGroup.add(earth)
  let earthTextureDisposed = false
  new THREE.TextureLoader().load('/textures/earth-day-blue-marble.jpg', texture => {
    if (earthTextureDisposed) { texture.dispose(); return }
    texture.colorSpace = THREE.SRGBColorSpace
    texture.wrapS = THREE.RepeatWrapping
    texture.anisotropy = Math.min(8, renderer.capabilities.getMaxAnisotropy())
    const material = earth.material as THREE.MeshStandardMaterial
    const previousMap = material.map
    const previousBump = material.bumpMap
    if (previousMap && previousMap !== previousBump) previousMap.dispose()
    if (previousBump) previousBump.dispose()
    material.bumpMap = texture
    material.map = texture
    material.bumpScale = .075
    material.needsUpdate = true
  })

  const clouds = new THREE.Mesh(
    new THREE.SphereGeometry(earthRadius * 1.018, sphereSegments, sphereSegments),
    new THREE.MeshPhongMaterial({ map: surfaceCloudFallback, alphaMap: surfaceCloudFallback, transparent: true, opacity: .82, depthWrite: false, color: 0xe9fff7, shininess: 18 }),
  )
  clouds.rotation.z = -.22
  clouds.visible = true
  earthGroup.add(clouds)
  const cloudHighlight = new THREE.Mesh(
    new THREE.SphereGeometry(earthRadius * 1.034, sphereSegments, sphereSegments),
    new THREE.MeshBasicMaterial({ map: surfaceCloudFallback, alphaMap: surfaceCloudFallback, transparent: true, opacity: .25, depthWrite: false, color: 0xb5fff0, blending: THREE.AdditiveBlending }),
  )
  cloudHighlight.rotation.z = -.22
  cloudHighlight.visible = true
  earthGroup.add(cloudHighlight)
  let surfaceCloudTextureDisposed = false
  new THREE.TextureLoader().load('/textures/earth-clouds-alpha.png', texture => {
    if (surfaceCloudTextureDisposed) { texture.dispose(); return }
    texture.colorSpace = THREE.SRGBColorSpace
    texture.wrapS = THREE.RepeatWrapping
    texture.anisotropy = Math.min(8, renderer.capabilities.getMaxAnisotropy())
    const material = clouds.material as THREE.MeshPhongMaterial
    const previousMap = material.map
    const previousAlpha = material.alphaMap
    if (previousMap && previousMap !== previousAlpha) previousMap.dispose()
    if (previousAlpha) previousAlpha.dispose()
    material.map = texture
    material.alphaMap = texture
    material.opacity = .62
    material.needsUpdate = true
    const highlight = cloudHighlight.material as THREE.MeshBasicMaterial
    highlight.map = texture
    highlight.alphaMap = texture
    highlight.needsUpdate = true
  })

  const atmosphereMaterial = new THREE.ShaderMaterial({
    transparent: true,
    side: THREE.BackSide,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
    uniforms: { uOpacity: { value: .16 } },
    vertexShader: `varying vec3 vNormal; void main(){ vNormal = normalize(normalMatrix * normal); gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }`,
    fragmentShader: `uniform float uOpacity; varying vec3 vNormal; void main(){ float rim = pow(max(0.0, 0.72 - dot(vNormal, vec3(0.0, 0.0, 1.0))), 2.2); gl_FragColor = vec4(0.42, 0.68, 0.76, rim * uOpacity); }`,
  })
  const atmosphere = new THREE.Mesh(
    new THREE.SphereGeometry(earthRadius * 1.05, sphereSegments, sphereSegments),
    atmosphereMaterial,
  )
  earthGroup.add(atmosphere)

  // Clouds continue as a separate atmospheric layer after the Earth leaves
  // frame, so the close-up resolves into a soft cloud field rather than black.
  const cloudField = new THREE.Group()
  cloudField.position.set(0, .15, -1.35)
  cloudField.visible = false
  const cloudFieldMaterials: THREE.MeshBasicMaterial[] = []
  for (let index = 0; index < (mobile ? 3 : 4); index += 1) {
    const material = new THREE.MeshBasicMaterial({
      map: backdropCloudTexture,
      alphaMap: backdropCloudTexture,
      transparent: true,
      depthWrite: false,
      opacity: .18 + index * .035,
      color: index % 2 ? 0xcbe9e5 : 0xeaf8f2,
      blending: THREE.AdditiveBlending,
    })
    const layer = new THREE.Mesh(new THREE.PlaneGeometry(22 + index * 5, 10 + index * 2.5), material)
    layer.position.set((index - 1.5) * 2.6, (index % 2 ? -.55 : .45) + index * .08, -index * .32)
    layer.rotation.z = (index - 1.5) * .045
    cloudField.add(layer)
    cloudFieldMaterials.push(material)
  }
  scene.add(cloudField)

  const logoTexture = createLogoTexture(mobile ? 256 : 512)
  const chipFaceTexture = createChipFaceTexture(mobile ? 512 : 1024)
  const node = new THREE.Group()
  node.visible = false
  const whiteMetal = new THREE.MeshPhysicalMaterial({ color: 0xf0f1ed, metalness: .82, roughness: .2, clearcoat: .85, clearcoatRoughness: .12 })
  const darkMetal = new THREE.MeshPhysicalMaterial({ color: 0x050607, metalness: .75, roughness: .3, clearcoat: .5 })
  const matteWhite = new THREE.MeshStandardMaterial({ color: 0xd8dad7, metalness: .36, roughness: .48 })
  const lineMaterial = new THREE.LineBasicMaterial({ color: 0x080a0b, transparent: true, opacity: .92 })

  const nodeCore = createChamferedPlate(2.5, .25, .44, whiteMetal)
  const rearPlate = createChamferedPlate(2.72, .3, .16, matteWhite)
  rearPlate.position.z = -.29
  const nodeInset = createChamferedPlate(2.08, .22, .09, darkMetal)
  nodeInset.position.z = .3
  const facePlate = createChamferedPlate(1.72, .18, .035, matteWhite)
  facePlate.position.z = .38
  const chipFace = new THREE.Mesh(
    new THREE.PlaneGeometry(1.48, 1.48),
    new THREE.MeshStandardMaterial({ map: chipFaceTexture, emissive: 0x252827, emissiveIntensity: .16, metalness: .25, roughness: .44, transparent: true }),
  )
  chipFace.position.z = .425
  const outerEdge = new THREE.LineSegments(new THREE.EdgesGeometry(nodeCore.geometry), new THREE.LineBasicMaterial({ color: 0xffffff, transparent: true, opacity: .82 }))
  const innerEdge = new THREE.LineSegments(new THREE.EdgesGeometry(nodeInset.geometry), new THREE.LineBasicMaterial({ color: 0x070809, transparent: true, opacity: .95 }))
  innerEdge.position.copy(nodeInset.position)

  const circuitLines = new THREE.Group()
  const routes: Array<Array<[number, number]>> = [
    [[-.66, .6], [-.18, .6], [.02, .32], [.62, .32]],
    [[-.7, .25], [-.36, .25], [-.02, -.06], [.7, -.06]],
    [[-.7, -.18], [-.24, -.18], [.08, -.5], [.64, -.5]],
    [[-.48, .7], [-.48, .38], [-.12, .02], [-.12, -.7]],
    [[.38, .68], [.38, .38], [.02, .02], [.02, -.68]],
    [[.68, .5], [.5, .5], [.22, .2], [.22, -.65]],
  ]
  for (const route of routes) circuitLines.add(createChipLine(route, .407, lineMaterial))
  const hub = new THREE.Mesh(new THREE.CylinderGeometry(.17, .17, .045, 32), darkMetal)
  hub.rotation.x = Math.PI / 2
  hub.position.set(0, 0, .43)

  const fastenerGeometry = new THREE.CylinderGeometry(.075, .075, .055, 20)
  const fasteners = [[-.98, -.98], [.98, -.98], [-.98, .98], [.98, .98]]
    .map(([x, y]) => {
      const fastener = new THREE.Mesh(fastenerGeometry, darkMetal)
      fastener.rotation.x = Math.PI / 2
      fastener.position.set(x, y, .31)
      return fastener
    })

  const sweepMaterial = new THREE.MeshBasicMaterial({ color: 0xffffff, transparent: true, opacity: 0, depthWrite: false, blending: THREE.AdditiveBlending })
  const sweep = new THREE.Mesh(new THREE.PlaneGeometry(.16, 1.82), sweepMaterial)
  sweep.position.set(-.72, 0, .445)
  sweep.rotation.z = -.42
  const nodeGlow = new THREE.PointLight(0xe9fff8, 0, 7, 2)
  nodeGlow.position.set(-.55, .7, 1.7)

  node.add(rearPlate, nodeCore, nodeInset, facePlate, chipFace, outerEdge, innerEdge, circuitLines, hub, sweep, nodeGlow, ...fasteners)
  scene.add(node)

  let width = 1
  let height = 1
  let frame = 0
  let destroyed = false
  let stage: LoginTransitionStage = 'orbital-login'
  let stageStarted = performance.now()
  let completionTimer: number | undefined
  const transitionRoot = canvas.closest<HTMLElement>('.login-transition')
  const nodeScreenPosition = new THREE.Vector3()

  const resize = () => {
    const bounds = canvas.getBoundingClientRect()
    width = Math.max(1, bounds.width)
    height = Math.max(1, bounds.height)
    renderer.setSize(width, height, false)
    camera.aspect = width / height
    camera.fov = width < 560 ? 50 : 42
    camera.updateProjectionMatrix()
  }

  const emitComplete = (fallback = false) => {
    if (destroyed || stage === 'complete') return
    stage = fallback ? 'fallback' : 'complete'
    options.onStage?.(stage)
    options.onComplete?.(fallback)
  }

  const setStage = (next: LoginTransitionStage) => {
    if (destroyed) return
    if (completionTimer) window.clearTimeout(completionTimer)
    completionTimer = undefined
    stage = next
    stageStarted = performance.now()
    options.onStage?.(next)
    if (next === 'node-reveal' && reduced) completionTimer = window.setTimeout(() => emitComplete(false), 60)
    if (next === 'page-enter') completionTimer = window.setTimeout(() => emitComplete(false), reduced ? 20 : PAGE_ENTER_MS)
  }

  const render = (time: number) => {
    if (destroyed) return
    const elapsed = time - stageStarted
    const revealProgress = easeOutCubic(elapsed / NODE_REVEAL_MS)
    const fallProgress = easeInOutCubic(elapsed / NODE_FALL_MS)
    const pageProgress = easeOutCubic(elapsed / PAGE_ENTER_MS)

    stars.rotation.y = time * .000008
    earth.rotation.y = time * .000035
    clouds.visible = true
    cloudHighlight.visible = true
    clouds.rotation.y = time * .000052
    cloudHighlight.rotation.y = -time * .000032
    cloudHighlight.rotation.x = Math.sin(time * .00011) * .035
    cloudField.rotation.z = Math.sin(time * .00008) * .025
    cloudField.position.x = Math.sin(time * .00012) * .42
    cloudField.position.y = .15 + Math.cos(time * .00017) * .14
    cloudFieldMaterials.forEach((material, index) => {
      material.opacity = (.16 + index * .035) + Math.sin(time * (.00034 + index * .00004) + index) * .025
    })
    if (stage === 'node-reveal') {
      node.visible = true
      cloudField.visible = true
      earthGroup.visible = revealProgress < .48
      const earthFade = 1 - clamp((revealProgress - .04) / .44)
      ;(earth.material as THREE.MeshStandardMaterial).opacity = earthFade
      ;(clouds.material as THREE.MeshPhongMaterial).opacity = .82 * earthFade
      ;(cloudHighlight.material as THREE.MeshBasicMaterial).opacity = .25 * earthFade
      atmosphereMaterial.uniforms.uOpacity.value = .16 * earthFade
      cloudFieldMaterials.forEach((material, index) => { material.opacity = (.16 + index * .035) * clamp((revealProgress - .18) / .3) })
      node.position.set(0, .2 + (1 - revealProgress) * .8, .3)
      node.scale.setScalar(.38 + revealProgress * .62)
      sweep.position.x = -.75 + revealProgress * 1.5
      sweepMaterial.opacity = .48 * Math.sin(revealProgress * Math.PI)
      nodeGlow.intensity = 16 * Math.sin(revealProgress * Math.PI)
      node.rotation.set(-.32 + revealProgress * .16, .65 + time * .00016, .12 + time * .00008)
      camera.position.z = 10 - revealProgress * 1.25
      camera.lookAt(0, -.55 + revealProgress * .35, 0)
      clouds.material.opacity = .78 + Math.sin(time * .0005) * .1
      cloudHighlight.material.opacity = .22 + Math.sin(time * .0007 + 1) * .06
      if (elapsed >= NODE_REVEAL_MS) setStage('node-fall')
    } else if (stage === 'node-fall') {
      node.visible = true
      cloudField.visible = true
      earthGroup.visible = false
      node.position.set(0, .2 - fallProgress * 7.4, .3 + fallProgress * 7.25)
      node.scale.setScalar(1 + fallProgress * 1.85)
      sweepMaterial.opacity = .26 * (1 - fallProgress)
      nodeGlow.intensity = 8 * (1 - fallProgress)
      node.rotation.x = -.16 + fallProgress * .78
      node.rotation.y += .008
      node.rotation.z += .004
      camera.position.z = 8.75 - fallProgress * .55
      earthGroup.position.y -= .0018
      if (elapsed >= NODE_FALL_MS) setStage('page-enter')
    } else if (stage === 'page-enter') {
      node.visible = true
      cloudField.visible = true
      earthGroup.visible = false
      renderer.toneMappingExposure = 1.05 + pageProgress * 2.2
      canvas.style.opacity = String(1 - pageProgress * .96)
    } else if (stage === 'orbital-login' || stage === 'authenticating') {
      node.visible = false
      sweepMaterial.opacity = 0
      nodeGlow.intensity = 0
      earthGroup.visible = true
      ;(earth.material as THREE.MeshStandardMaterial).opacity = 1
      ;(clouds.material as THREE.MeshPhongMaterial).opacity = .82
      ;(cloudHighlight.material as THREE.MeshBasicMaterial).opacity = .25
      atmosphereMaterial.uniforms.uOpacity.value = .16
      cloudField.visible = false
      camera.position.z += (10 - camera.position.z) * .06
      renderer.toneMappingExposure = 1.05
      canvas.style.opacity = '1'
    }

    camera.lookAt(0, -.15, 0)
    if (node.visible && transitionRoot) {
      camera.updateMatrixWorld()
      node.updateWorldMatrix(true, false)
      nodeScreenPosition.setFromMatrixPosition(node.matrixWorld).project(camera)
      transitionRoot.style.setProperty('--node-screen-x', `${(nodeScreenPosition.x * .5 + .5) * width}px`)
      transitionRoot.style.setProperty('--node-screen-y', `${(-nodeScreenPosition.y * .5 + .5) * height}px`)
      transitionRoot.style.setProperty('--node-copy-scale', String(Math.min(1.24, .9 + node.scale.x * .12)))
      transitionRoot.style.setProperty('--node-copy-rotate-x', `${THREE.MathUtils.radToDeg(node.rotation.x) * .13}deg`)
      transitionRoot.style.setProperty('--node-copy-rotate-y', `${Math.sin(node.rotation.y) * 4.5}deg`)
      transitionRoot.style.setProperty('--node-copy-rotate-z', `${Math.sin(node.rotation.z) * 2.5}deg`)
    }
    renderer.render(scene, camera)
    frame = window.requestAnimationFrame(render)
  }

  resize()
  window.addEventListener('resize', resize, { passive: true })
  frame = window.requestAnimationFrame(render)
  options.onStage?.('orbital-login')

  return {
    setStage,
    destroy() {
      destroyed = true
      if (frame) window.cancelAnimationFrame(frame)
      if (completionTimer) window.clearTimeout(completionTimer)
      window.removeEventListener('resize', resize)
      earthTextureDisposed = true
      surfaceCloudTextureDisposed = true
      transitionRoot?.style.removeProperty('--node-screen-x')
      transitionRoot?.style.removeProperty('--node-screen-y')
      transitionRoot?.style.removeProperty('--node-copy-scale')
      transitionRoot?.style.removeProperty('--node-copy-rotate-x')
      transitionRoot?.style.removeProperty('--node-copy-rotate-y')
      transitionRoot?.style.removeProperty('--node-copy-rotate-z')
      disposeObject(scene)
      renderer.renderLists.dispose()
      renderer.dispose()
      renderer.forceContextLoss()
      canvas.style.opacity = ''
    },
  }
}
