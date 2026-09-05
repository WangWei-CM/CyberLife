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

function createEarthTexture(quality: number) {
  return canvasTexture(quality, quality / 2, context => {
    const width = context.canvas.width
    const height = context.canvas.height
    const ocean = context.createLinearGradient(0, 0, 0, height)
    ocean.addColorStop(0, '#183d4d')
    ocean.addColorStop(.52, '#0b2939')
    ocean.addColorStop(1, '#061a29')
    context.fillStyle = ocean
    context.fillRect(0, 0, width, height)

    for (let index = 0; index < 54; index += 1) {
      const x = seeded(index + 10) * width
      const y = seeded(index + 90) * height
      const radiusX = width * (.018 + seeded(index + 130) * .055)
      const radiusY = height * (.025 + seeded(index + 170) * .095)
      context.save()
      context.translate(x, y)
      context.rotate(seeded(index + 210) * Math.PI)
      context.fillStyle = index % 3 === 0 ? 'rgba(87, 128, 111, .82)' : 'rgba(57, 105, 96, .9)'
      context.beginPath()
      for (let point = 0; point < 12; point += 1) {
        const angle = point / 12 * TAU
        const radius = .68 + seeded(index * 30 + point) * .42
        const px = Math.cos(angle) * radiusX * radius
        const py = Math.sin(angle) * radiusY * radius
        if (point === 0) context.moveTo(px, py)
        else context.lineTo(px, py)
      }
      context.closePath()
      context.fill()
      context.restore()
    }

    context.globalCompositeOperation = 'screen'
    const polar = context.createLinearGradient(0, 0, 0, height)
    polar.addColorStop(0, 'rgba(210, 233, 225, .42)')
    polar.addColorStop(.12, 'rgba(210, 233, 225, 0)')
    polar.addColorStop(.88, 'rgba(210, 233, 225, 0)')
    polar.addColorStop(1, 'rgba(210, 233, 225, .34)')
    context.fillStyle = polar
    context.fillRect(0, 0, width, height)
  })
}

function createCloudTexture(quality: number) {
  return canvasTexture(quality, quality / 2, context => {
    const width = context.canvas.width
    const height = context.canvas.height
    context.clearRect(0, 0, width, height)
    context.filter = `blur(${Math.max(5, quality / 150)}px)`
    for (let index = 0; index < 90; index += 1) {
      const x = seeded(index + 310) * width
      const y = seeded(index + 390) * height
      const radiusX = width * (.008 + seeded(index + 470) * .04)
      const radiusY = height * (.008 + seeded(index + 520) * .025)
      const alpha = .14 + seeded(index + 610) * .3
      context.fillStyle = `rgba(231, 244, 239, ${alpha})`
      context.beginPath()
      context.ellipse(x, y, radiusX, radiusY, seeded(index + 690) * Math.PI, 0, TAU)
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

function disposeObject(root: THREE.Object3D) {
  root.traverse(object => {
    const mesh = object as THREE.Mesh
    mesh.geometry?.dispose()
    const materials = mesh.material ? (Array.isArray(mesh.material) ? mesh.material : [mesh.material]) : []
    for (const material of materials) {
      for (const value of Object.values(material)) {
        if (value instanceof THREE.Texture) value.dispose()
      }
      material.dispose()
    }
  })
}

/** Original CyberLife WebGL scene. No third-party site assets are embedded. */
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
  const rimLight = new THREE.PointLight(0x55cfc5, 35, 24, 2)
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

  const earthGroup = new THREE.Group()
  earthGroup.position.set(mobile ? 2.2 : 3.35, mobile ? -3.2 : -2.35, -1.8)
  earthGroup.rotation.z = -.16
  scene.add(earthGroup)

  const textureQuality = mobile ? 768 : 1536
  const earthTexture = createEarthTexture(textureQuality)
  const cloudTexture = createCloudTexture(textureQuality)
  const sphereSegments = mobile ? 48 : 72
  const earth = new THREE.Mesh(
    new THREE.SphereGeometry(3.35, sphereSegments, sphereSegments),
    new THREE.MeshStandardMaterial({ map: earthTexture, roughness: .82, metalness: .04, color: 0x83b4b1 }),
  )
  earth.rotation.z = -.22
  earthGroup.add(earth)

  const clouds = new THREE.Mesh(
    new THREE.SphereGeometry(3.42, sphereSegments, sphereSegments),
    new THREE.MeshPhongMaterial({ map: cloudTexture, alphaMap: cloudTexture, transparent: true, opacity: .54, depthWrite: false, color: 0xdaf3ec }),
  )
  clouds.rotation.z = -.22
  earthGroup.add(clouds)

  const atmosphere = new THREE.Mesh(
    new THREE.SphereGeometry(3.54, sphereSegments, sphereSegments),
    new THREE.ShaderMaterial({
      transparent: true,
      side: THREE.BackSide,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
      vertexShader: `varying vec3 vNormal; void main(){ vNormal = normalize(normalMatrix * normal); gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }`,
      fragmentShader: `varying vec3 vNormal; void main(){ float rim = pow(0.72 - dot(vNormal, vec3(0.0, 0.0, 1.0)), 2.2); gl_FragColor = vec4(0.23, 0.86, 0.82, rim * 0.48); }`,
    }),
  )
  earthGroup.add(atmosphere)

  const orbitGroup = new THREE.Group()
  orbitGroup.position.copy(earthGroup.position)
  orbitGroup.rotation.set(.28, 0, -.34)
  const orbitA = createOrbit(4.45, 0x6edbd0, .3)
  const orbitB = createOrbit(5.15, 0x92b8b7, .17)
  orbitB.rotation.x = .24
  orbitB.rotation.z = .16
  orbitGroup.add(orbitA, orbitB)
  scene.add(orbitGroup)

  const logoTexture = createLogoTexture(mobile ? 256 : 512)
  const logoMaterial = new THREE.SpriteMaterial({ map: logoTexture, transparent: true, opacity: .92, depthWrite: false, blending: THREE.AdditiveBlending })
  const orbitalLogo = new THREE.Sprite(logoMaterial)
  orbitalLogo.position.set(0, .25, 1.2)
  orbitalLogo.scale.set(2.3, 2.3, 1)
  scene.add(orbitalLogo)

  const node = new THREE.Group()
  node.visible = false
  const nodeCore = new THREE.Mesh(
    new THREE.BoxGeometry(2.25, 2.25, .55, 6, 6, 2),
    new THREE.MeshPhysicalMaterial({ color: 0x87999a, metalness: .94, roughness: .24, clearcoat: .7, clearcoatRoughness: .18 }),
  )
  const nodeInset = new THREE.Mesh(
    new THREE.BoxGeometry(1.78, 1.78, .08),
    new THREE.MeshPhysicalMaterial({ color: 0x142328, metalness: .82, roughness: .34, clearcoat: .45 }),
  )
  nodeInset.position.z = .32
  const nodeMark = new THREE.Mesh(
    new THREE.PlaneGeometry(1.36, 1.36),
    new THREE.MeshBasicMaterial({ map: logoTexture, transparent: true, opacity: .9, depthWrite: false }),
  )
  nodeMark.position.z = .38
  const edge = new THREE.LineSegments(new THREE.EdgesGeometry(nodeCore.geometry), new THREE.LineBasicMaterial({ color: 0xc9e9e3, transparent: true, opacity: .74 }))
  node.add(nodeCore, nodeInset, nodeMark, edge)
  scene.add(node)

  let width = 1
  let height = 1
  let frame = 0
  let destroyed = false
  let stage: LoginTransitionStage = 'orbital-login'
  let stageStarted = performance.now()
  let completionTimer: number | undefined

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
    clouds.rotation.y = time * .000052
    orbitGroup.rotation.y = Math.sin(time * .00009) * .08
    orbitalLogo.position.y = .25 + Math.sin(time * .0007) * .08
    orbitalLogo.material.opacity = stage === 'authenticating' ? .38 : stage === 'orbital-login' ? .92 : Math.max(0, 1 - revealProgress * 1.8)

    if (stage === 'node-reveal') {
      node.visible = true
      node.position.set(0, .2 + (1 - revealProgress) * .8, .3)
      node.scale.setScalar(.38 + revealProgress * .62)
      node.rotation.set(-.32 + revealProgress * .16, .65 + time * .00016, .12 + time * .00008)
      camera.position.z = 10 - revealProgress * 1.25
      if (elapsed >= NODE_REVEAL_MS) setStage('node-fall')
    } else if (stage === 'node-fall') {
      node.visible = true
      node.position.set(0, .2 - fallProgress * 7.4, .3 + fallProgress * 7.25)
      node.scale.setScalar(1 + fallProgress * 1.85)
      node.rotation.x = -.16 + fallProgress * .78
      node.rotation.y += .008
      node.rotation.z += .004
      camera.position.z = 8.75 - fallProgress * .55
      earthGroup.position.y -= .0018
      if (elapsed >= NODE_FALL_MS) setStage('page-enter')
    } else if (stage === 'page-enter') {
      node.visible = true
      renderer.toneMappingExposure = 1.05 + pageProgress * 2.2
      canvas.style.opacity = String(1 - pageProgress * .96)
    } else if (stage === 'orbital-login' || stage === 'authenticating') {
      node.visible = false
      camera.position.z += (10 - camera.position.z) * .06
      renderer.toneMappingExposure = 1.05
      canvas.style.opacity = '1'
    }

    camera.lookAt(0, -.15, 0)
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
      disposeObject(scene)
      renderer.renderLists.dispose()
      renderer.dispose()
      renderer.forceContextLoss()
      canvas.style.opacity = ''
    },
  }
}
