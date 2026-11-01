export type Health = { status: string; score: number; issues: string[] };

export type CPUInfo = {
  brand: string;
  cores: number;
  threads: number;
  baseGhz: number;
  maxGhz: number;
  freqGhz: number;
  usage: number;
  temp: number;
  power: number;
  perCore: number[];
  sensors: { label: string; value: number }[];
  socket: string;
  cache: string;
};

export type MemoryInfo = {
  total: number;
  used: number;
  free: number;
  available: number;
  cached: number;
  buffers: number;
  swapTotal: number;
  swapUsed: number;
  usage: number;
  type: string;
  speed: string;
};

export type SystemInfo = {
  hostname: string;
  name: string;
  version: string;
  kernel: string;
  board: string;
  cpu: CPUInfo;
  memory: MemoryInfo;
  uptime: number;
  bootTime: string;
  load: number[];
  temperatures: { label: string; value: number; max: number }[];
  fans: { label: string; rpm: number }[];
  health: Health;
  license: string;
  flashGuid: string;
  caseModel: string;
};

export type ParityCheck = {
  date: string;
  duration: number;
  speed: number;
  status: string;
  errors: number;
  action: string;
  size: number;
};

export type Parity = {
  configured: boolean;
  disks: string[];
  running: boolean;
  paused: boolean;
  progress: number;
  position: number;
  speed: number;
  errors: number;
  elapsed: number;
  lastCheck: ParityCheck | null;
  history: ParityCheck[];
};

export type ArrayStartOptions = {
  keyType: "none" | "passphrase" | "keyfile";
  key: string;
  mode: "normal" | "maintenance";
};

export type MoverRun = {
  started: string;
  finished: string;
  duration: number;
  total: number;
  moved: number;
  status: string;
};

export type MoverStatus = {
  running: boolean;
  total: number;
  remain: number;
  progress: number;
  throughput: number;
  eta: number;
  shareTotal: number;
  shareRemain: number;
  shareThroughput: number;
  shareEta: number;
  file: string;
  action: string;
  filePercent: number;
  pid: number;
  updated: string;
  history: MoverRun[];
};

export type ArrayInfo = {
  state: string;
  protection: string;
  status: string;
  total: number;
  used: number;
  free: number;
  usage: number;
  disks: number;
  parity: Parity;
  sync: boolean;
  startedAt: string;
};

export type Disk = {
  id: string;
  name: string;
  device: string;
  type: string;
  pool: string;
  fsType: string;
  size: number;
  used: number;
  free: number;
  temp: number;
  status: string;
  smart: string;
  errors: number;
  readBps: number;
  writeBps: number;
  rotational: boolean;
  serial: string;
  model: string;
  firmware: string;
  spinupGroup: string;
  luks: boolean;
  warnings: string[];
  position: number;
  lastSpinUp: string;
  color: string;
};

export type Share = {
  name: string;
  comment: string;
  size: number;
  free: number;
  used: number;
  usage: number;
  allocator: string;
  splitLevel: string;
  cacheMode: string;
  pool: string;
  security: string;
  smb: boolean;
  nfs: boolean;
  exclusive: boolean;
  include: string[];
  exclude: string[];
  lastModified: string;
};

export type Pool = {
  name: string;
  fsType: string;
  devices: string[];
  size: number;
  used: number;
  free: number;
  usage: number;
  status: string;
  cache: boolean;
};

export type Port = { host: number; container: number; protocol: string; label?: string };

export type ContainerEnv = { key: string; value: string };

export type ContainerVolume = {
  source: string;
  destination: string;
  readOnly: boolean;
};

export type ContainerTemplate = {
  id: string;
  name: string;
  image: string;
  network: string;
  restartPolicy: string;
  icon: string;
  webUi: string;
  privileged: boolean;
  ports: Port[];
  volumes: ContainerVolume[];
  env: ContainerEnv[];
  modified: string;
};

export type ContainerCreateOptions = {
  name: string;
  image: string;
  networkMode: string;
  restartPolicy: string;
  ports: Port[];
  volumes: ContainerVolume[];
  env: ContainerEnv[];
  icon: string;
  webUi: string;
  privileged: boolean;
};

export type ContainerUpdateEvent = {
  step: string;
  message: string;
  progress: number;
  done: boolean;
  error: boolean;
};

export type ImageCheck = {
  image: string;
  local: string;
  remote: string;
  update: boolean;
  error?: string;
};

export type ContainerConfig = {
  id: string;
  name: string;
  image: string;
  restartPolicy: string;
  networkMode: string;
  ports: Port[];
  volumes: ContainerVolume[];
  env: ContainerEnv[];
};

export type Container = {
  id: string;
  name: string;
  image: string;
  repository: string;
  tag: string;
  version: string;
  state: string;
  status: string;
  uptime: number;
  autostart: boolean;
  network: string;
  ip: string;
  ports: Port[];
  cpu: number;
  memory: number;
  memoryLimit: number;
  networkRx: number;
  networkTx: number;
  color: string;
  category: string;
  updateAvailable: boolean;
  created: string;
  icon: string;
  webUi: string;
  restartPolicy: string;
};

export type VM = {
  id: string;
  name: string;
  state: string;
  os: string;
  vcpus: number;
  memory: number;
  cpuUsage: number;
  memoryUsage: number;
  diskSize: number;
  diskUsed: number;
  uptime: number;
  autostart: boolean;
  ip: string;
  machine: string;
  color: string;
  icon: string;
  primaryGpu: string;
  lastStarted: string;
  consoleProtocol: string;
  consolePort: number;
  consoleWebSocket: number;
  consoleKeyboard: boolean;
};

export type Notification = {
  id: string;
  title: string;
  subject: string;
  description: string;
  importance: "normal" | "warning" | "alert" | string;
  timestamp: string;
  read: boolean;
  archived: boolean;
  source: string;
};

export type NetInterface = {
  name: string;
  description: string;
  mac: string;
  ip: string;
  ipv6: string;
  speedMbps: number;
  state: string;
  rxBps: number;
  txBps: number;
  rxTotal: number;
  txTotal: number;
  dhcp: boolean;
  gateway: string;
  dns: string[];
  bond: string;
  bridge: string;
};

export type WireGuardPeer = {
  name: string;
  publicKey: string;
  endpoint: string;
  allowedIps: string[];
  latestHandshake: string;
  rx: number;
  tx: number;
  online: boolean;
};

export type NetworkInfo = {
  interfaces: NetInterface[];
  gateway: string;
  dns: string[];
  hostname: string;
  domain: string;
  wireGuard: { enabled: boolean; port: number; peers: WireGuardPeer[] };
  services: { name: string; port: number; protocol: string; status: string; url: string }[];
};

export type User = {
  name: string;
  description: string;
  uid: number;
  role: string;
  shares: string[];
  lastActive: string;
  root: boolean;
  color: string;
};

export type PluginEvent = {
  line: string;
  done: boolean;
  error: boolean;
};

export type Plugin = {
  name: string;
  version: string;
  latest: string;
  author: string;
  description: string;
  category: string;
  updateAvailable: boolean;
  enabled: boolean;
  color: string;
};

export type SmartAttribute = {
  id: number;
  name: string;
  value: number;
  worst: number;
  threshold: number;
  raw: string;
  failing: boolean;
};

export type SmartReport = {
  device: string;
  model: string;
  serial: string;
  firmware: string;
  passed: boolean;
  temperature: number;
  powerOnHours: number;
  selfTest: string;
  attributes: SmartAttribute[];
  available: boolean;
  message: string;
};

export type GPUProcess = { pid: number; name: string; memory: number };

export type GPU = {
  index: number;
  uuid: string;
  name: string;
  vendor: string;
  driver: string;
  memoryTotal: number;
  memoryUsed: number;
  utilization: number;
  temperature: number;
  power: number;
  powerLimit: number;
  fan: number;
  clockMhz: number;
  memoryClockMhz: number;
  processes: GPUProcess[];
};

export type UPS = {
  present: boolean;
  model: string;
  status: string;
  battery: number;
  load: number;
  runtime: number;
  inputVoltage: number;
  outputVoltage: number;
  nominalPower: number;
  lastEvent: string;
  onBattery: boolean;
};

export type Settings = {
  theme: "dark" | "light" | "system" | string;
  accent: "ember" | "violet" | "cyan" | "emerald" | "amber" | string;
  density: "comfortable" | "compact" | string;
  refreshInterval: number;
  language: string;
  serverName: string;
  autoUpdate: boolean;
  emailAlerts: boolean;
  pushAlerts: boolean;
  telemetry: boolean;
  temperatureUnit: "celsius" | "fahrenheit" | string;
};

export type LogLine = { time: string; level: string; source: string; message: string };

export type Metrics = {
  t: number;
  cpu: number;
  memory: number;
  netRx: number;
  netTx: number;
  diskRead: number;
  diskWrite: number;
  temp: number;
  load: number;
  cores: number[];
};

export type MetricPoint = {
  t: number;
  cpu: number;
  memory: number;
  netRx: number;
  netTx: number;
  diskRead: number;
  diskWrite: number;
  temp: number;
};

export type Snapshot = {
  system: SystemInfo;
  array: ArrayInfo;
  disks: Disk[];
  shares: Share[];
  pools: Pool[];
  containers: Container[];
  vms: VM[];
  network: NetworkInfo;
  users: User[];
  plugins: Plugin[];
  notifications: Notification[];
  gpus: GPU[];
  ups: UPS;
  settings: Settings;
  metrics: Metrics;
  history: MetricPoint[];
  serverTime: string;
};

export type SessionInfo = {
  authenticated: boolean;
  user: string;
};

export type ActionKind =
  | "array.start"
  | "array.stop"
  | "array.pause"
  | "disk.spinup"
  | "disk.spindown"
  | "container.start"
  | "container.stop"
  | "container.restart"
  | "container.update"
  | "vm.start"
  | "vm.stop"
  | "vm.pause"
  | "vm.resume"
  | "parity.start"
  | "parity.stop"
  | "parity.pause"
  | "parity.resume"
  | "notification.ack"
  | "notification.ackall"
  | "notification.archive";
