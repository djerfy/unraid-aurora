export type Health = { status: string; score: number; issues: string[] };

export type ServerHealth = {
  status: string;
  version: string;
  provider: string;
  uptime: number;
  time: string;
};

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
  files: string[];
};

export type MoverStatus = {
  running: boolean;
  progressEnabled: boolean;
  progressSource: string;
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
  phase: string;
  files: string[];
  history: MoverRun[];
};

export type ArrayRebuild = {
  running: boolean;
  action: string;
  progress: number;
  position: number;
  size: number;
  speed: number;
  errors: number;
  elapsed: number;
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
  rebuild: ArrayRebuild;
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
  brand: string;
  model: string;
  media: string;
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
  usageUpdated?: string | null;
};

export type ShareConfig = {
  name: string;
  comment: string;
  floor: string;
  allocator: string;
  splitLevel: string;
  include: string[];
  exclude: string[];
  useCache: string;
  cachePool: string;
  cachePool2: string;
  cow: string;
  exclusive: boolean;
  smbExport: string;
  smbSecurity: string;
  smbCaseSensitive: string;
  smbVolumeSizeLimit: string;
  nfsExport: string;
  nfsSecurity: string;
  nfsHostList: string;
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

export type ContainerConnection = {
  protocol: string;
  state: string;
  localAddress: string;
  localPort: number;
  remoteAddress: string;
  remotePort: number;
  process: string;
  pid: number;
  country: string;
  city: string;
};

export type ContainerConnections = {
  available: boolean;
  reason?: string;
  connections: ContainerConnection[];
};

export type ContainerMount = {
  type: string;
  source: string;
  destination: string;
  readOnly: boolean;
};

export type ContainerDevice = {
  hostPath: string;
  containerPath: string;
  permissions: string;
};

export type ContainerUlimit = {
  name: string;
  soft: number;
  hard: number;
};

export type ContainerAdvanced = {
  macAddress: string;
  hostname: string;
  devices: ContainerDevice[];
  capAdd: string[];
  capDrop: string[];
  sysctls: Record<string, string>;
  ulimits: ContainerUlimit[];
  labels: Record<string, string>;
  extraHosts: string[];
  tmpfs: string[];
  dns: string[];
  dnsSearch: string[];
  memoryLimit: number;
  nanoCpus: number;
  cpuSet: string;
  pidsLimit: number;
  readOnlyRootfs: boolean;
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
  advanced: ContainerAdvanced;
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
  advanced: ContainerAdvanced;
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
  advanced: ContainerAdvanced;
};

export type ContainerVulnerability = {
  id: string;
  package: string;
  installedVersion: string;
  fixedVersion: string;
  severity: string;
  title: string;
  target: string;
  type: string;
  url: string;
};

export type ContainerSecurity = {
  enabled: boolean;
  available: boolean;
  scanning: boolean;
  stale: boolean;
  scannedAt?: string;
  trivyVersion?: string;
  dbUpdatedAt?: string;
  critical: number;
  high: number;
  medium: number;
  low: number;
  unknown: number;
  total: number;
  error?: string;
  image?: string;
  imageId?: string;
};

export type ContainerSecurityDetail = ContainerSecurity & {
  vulnerabilities: ContainerVulnerability[];
};

export type SecurityContainerRef = {
  id: string;
  name: string;
  state: string;
};

export type SecurityImage = {
  image: string;
  imageId?: string;
  containers: SecurityContainerRef[];
  security: ContainerSecurity;
  vulnerabilities: ContainerVulnerability[];
};

export type SecurityTotals = {
  images: number;
  scannedImages: number;
  containers: number;
  scannedContainers: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
  unknown: number;
  total: number;
  scanning: number;
  failed: number;
  stale: number;
  lastScanAt?: string;
};

export type SecurityOverview = {
  enabled: boolean;
  images: SecurityImage[];
  totals: SecurityTotals;
};

export type TrivyStatus = {
  enabled: boolean;
  installed: boolean;
  runtimeReady: boolean;
  version: string;
  dbReady: boolean;
  javaDbReady: boolean;
  dbUpdatedAt?: string;
  dbSize: number;
  javaDbSize: number;
  dbUpdateHour: number;
  installing: boolean;
  updatingDb: boolean;
  stage?: string;
  error?: string;
};

export type TrivyVersion = {
  version: string;
  name: string;
  publishedAt: string;
  prerelease: boolean;
};

export type Container = {
  id: string;
  name: string;
  image: string;
  imageId: string;
  repository: string;
  tag: string;
  version: string;
  state: string;
  status: string;
  health: string;
  uptime: number;
  autostart: boolean;
  network: string;
  ip: string;
  ports: Port[];
  mounts: ContainerMount[];
  cpu: number;
  memory: number;
  memoryLimit: number;
  networkRx: number;
  networkTx: number;
  networkRxRate: number;
  networkTxRate: number;
  blockRead: number;
  blockWrite: number;
  blockReadRate: number;
  blockWriteRate: number;
  color: string;
  category: string;
  updateAvailable: boolean;
  created: string;
  icon: string;
  webUi: string;
  restartPolicy: string;
  security: ContainerSecurity;
};

export type VMCreateOptions = {
  name: string;
  vcpus: number;
  memoryMB: number;
  diskGB: number;
  isoPath: string;
  network: string;
  start: boolean;
};

export type VMSnapshot = {
  name: string;
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

export type AppCatalogEntry = {
  id: string;
  name: string;
  kind: "docker" | "plugin" | string;
  repository?: string;
  registry?: string;
  network?: string;
  icon?: string;
  webUi?: string;
  overview?: string;
  category?: string;
  categories: string[];
  support?: string;
  project?: string;
  readMe?: string;
  templateUrl?: string;
  pluginUrl?: string;
  author?: string;
  repo?: string;
  date?: string;
  requires?: string;
  extraParams?: string;
  privileged: boolean;
  downloads: number;
  pluginVersion?: string;
};

export type AppCatalogPage = {
  entries: AppCatalogEntry[];
  total: number;
  categories: string[];
  updatedAt?: string;
  cached: boolean;
};

export type SchedulerJob = {
  mode: string;
  hour: number;
  minute: number;
  day: number;
  dayOfMonth: number;
  month: number;
  intervalHours: number;
};

export type ParitySchedule = SchedulerJob & { writeCorrections: boolean };
export type MoverSchedule = SchedulerJob & { logging: boolean; progress: boolean };
export type TrimSchedule = SchedulerJob;

export type SchedulerSettings = {
  parity: ParitySchedule;
  mover: MoverSchedule;
  trim: TrimSchedule;
};

export type SystemDisplaySettings = {
  dateFormat: string;
  timeFormat: string;
};

export type SystemSettings = {
  timeZone: string;
  ntpEnabled: boolean;
  ntpServers: string[];
  currentTime: string;
  display: SystemDisplaySettings;
};

export type SchedulerState = {
  settings: SchedulerSettings;
  system: SystemSettings;
};

export type SchedulerPreviewFile = {
  path: string;
  content: string;
  diff?: string;
  existed: boolean;
  delete: boolean;
};

export type SchedulerPreview = {
  files: SchedulerPreviewFile[];
  warnings: string[];
};

export type VMAdvancedConfig = {
  vcpus: number;
  memoryMB: number;
  cpuSet: string;
  networks: { bridge: string; model: string; mac: string }[];
  soundModel: string;
  hostDevices: { type: string; address?: string; vendor?: string; product?: string }[];
  sharedFolders: { source: string; target: string; readOnly: boolean }[];
};

export type VMHostDevice = {
  type: string;
  address?: string;
  vendor?: string;
  product?: string;
  name: string;
  driver?: string;
  iommuGroup?: string;
  class?: string;
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

export type UserCreateOptions = {
  name: string;
  description: string;
  password: string;
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

export type BalanceDisk = {
  name: string;
  mount: string;
  role: string;
  filesystem: string;
  size: number;
  used: number;
  free: number;
};

export type BalanceEntry = {
  name: string;
  path: string;
  dir: boolean;
  size: number;
  modified: string;
};

export type BalanceOptions = {
  source: string;
  paths: string[];
  destinations: string[];
  deleteSource: boolean;
  dryRun: boolean;
  overwrite: boolean;
};

export type BalanceMove = {
  path: string;
  destination: string;
  size: number;
  status: string;
  error?: string;
};

export type BalanceJob = {
  id: string;
  source: string;
  destinations: string[];
  mode: string;
  state: string;
  phase: string;
  started: string;
  finished: string;
  totalBytes: number;
  doneBytes: number;
  totalFiles: number;
  doneFiles: number;
  currentPath: string;
  speed: number;
  eta: number;
  dryRun: boolean;
  deleteSource: boolean;
  files: BalanceMove[];
  log: string[];
  errors: string[];
};

export type ConfigFileChange = {
  path: string;
  existed: boolean;
  size: number;
  diff: string;
};

export type ConfigHistory = {
  id: string;
  created: string;
  action: string;
  description: string;
  files: ConfigFileChange[];
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

export type StorageSection = {
  id: string;
  name: string;
  color: string;
  media: "any" | "hdd" | "ssd" | "nvme";
  rows: number;
  cols: number;
  assignments: Record<string, number>;
};

export type StorageView = {
  id: string;
  name: string;
  kind: "tower" | "server" | "nas";
  size: "s" | "m" | "l" | "custom";
  x: number;
  y: number;
  w: number;
  h: number;
  sections: StorageSection[];
};

export type StorageLayout = {
  version: number;
  views: StorageView[];
  updated: string;
};

export type Settings = {
  theme: "dark" | "light" | "system" | string;
  accent: "ember" | "violet" | "cyan" | "emerald" | "amber" | "blue" | string;
  density: "comfortable" | "compact" | string;
  refreshInterval: number;
  language: string;
  serverName: string;
  autoUpdate: boolean;
  emailAlerts: boolean;
  pushAlerts: boolean;
  discordWebhook: string;
  ntfyUrl: string;
  webhookUrl: string;
  webhookMethod: string;
  webhookHeaders: Record<string, string>;
  webhookBody: string;
  notifyMinImportance: string;
  notifyMutedSources: string[];
  notifyEvents: Record<string, boolean>;
  telemetry: boolean;
  temperatureUnit: "celsius" | "fahrenheit" | string;
  trivyEnabled: boolean;
  trivyVersion: string;
  trivyDbUpdateHour: number;
};

export type UPSSettings = {
  service: string;
  cable: string;
  type: string;
  device: string;
  batteryLevel: number;
  minutes: number;
  timeout: number;
  killUps: string;
  overrideCapacity: number;
};

export type ZFSDataset = {
  name: string;
  used: number;
  available: number;
  refer: number;
  mountpoint: string;
  type: string;
  compression: string;
  quota: number;
  readOnly: boolean;
  recordsize: string;
};

export type ZFSDatasetCreate = {
  name: string;
  compression: string;
  quotaBytes: number;
  recordsize: string;
  readOnly: boolean;
};

export type ZFSDatasetUpdate = {
  name: string;
  compression?: string;
  quotaBytes?: number;
  recordsize?: string;
  readOnly?: boolean;
};

export type ZFSPool = {
  name: string;
  size: number;
  allocated: number;
  free: number;
  health: string;
  datasets: ZFSDataset[];
};

export type ZFSSnapshot = {
  name: string;
  dataset: string;
  used: number;
  created: number;
};

export type UserScript = {
  id: string;
  name: string;
  description: string;
  schedule: string;
  frequency?: string;
  cron?: string;
};

export type APIToken = {
  id: string;
  name: string;
  role: "viewer" | "operator" | "admin";
  readOnly: boolean;
  created: string;
  lastUsed: string;
  expiresAt?: string | null;
  scopes?: string[];
};

export type AuditEntry = {
  time: string;
  user: string;
  method: string;
  path: string;
  status: number;
  remote: string;
};

export type LogLine = { time: string; level: string; source: string; message: string };

export type VMBackupFile = {
  name: string;
  kind: string;
  target?: string;
  format?: string;
  size: number;
  sha256: string;
};

export type VMBackup = {
  id: string;
  kind: string;
  vmId: string;
  vmName: string;
  dir: string;
  state: string;
  phase: string;
  message: string;
  startedAt: string;
  finishedAt?: string;
  totalBytes: number;
  doneBytes: number;
  files: VMBackupFile[];
  error?: string;
};

export type VMAutostartOrder = {
  order: string[];
  managed: boolean;
  managedVms: string[];
  updatedAt: string;
};

export type VMAutostartResult = {
  started: string[];
  skipped: string[];
  failed: { name: string; error: string }[];
};

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
  power: number;
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

export type OSStatus = {
  version: string;
  branch: string;
  latest?: string;
  latestName?: string;
  updateAvailable: boolean;
  eligible: boolean;
  changelog?: string;
  changelogPretty?: string;
  releaseDate?: string;
  checkedAt?: string;
  checkError?: string;
  rebootRequired: boolean;
  rebootType?: string;
  rebootVersion?: string;
  previousVersion?: string;
  previousDate?: string;
  restoreAvailable: boolean;
  thirdPartyDrivers: boolean;
};

export type UnraidAPIStatus = {
  installed: boolean;
  running: boolean;
  state: string;
  version: string;
  raw: string;
  config?: string;
  logLevel?: string;
};

export type NetworkInterfaceConfig = {
  index: number;
  interface: string;
  physicalNic: string;
  state: string;
  useDhcp: boolean;
  ipAddr: string;
  netmask: string;
  gateway: string;
  useMtu: boolean;
  mtu: number;
  bonding: boolean;
  bondName: string;
  bondNics: string[];
  bondMode: string;
  bondMiiMon: number;
  bridging: boolean;
  bridgeName: string;
  bridgeNics: string[];
  bridgeStp: boolean;
  bridgeFd: number;
  lockoutRisk: boolean;
};

export type NetworkConfiguration = {
  interfaces: NetworkInterfaceConfig[];
  dns: string[];
  availableNics: string[];
  format: string;
  backup?: string;
};

export type AuthSession = {
  nonce: string;
  user: string;
  ip: string;
  created: string;
  lastSeen: string;
  expires: string;
  current: boolean;
};

export type NUTSettings = {
  available: boolean;
  service: string;
  mode: string;
  name: string;
  driver: string;
  port: string;
  ipAddr: string;
  batteryLevel: number;
  runtimeValue: number;
  timeout: number;
  poll: number;
  upsKill: string;
  passwordSet: boolean;
  monitorPassword?: string;
};

export type Task = {
  id: string;
  name: string;
  category: string;
  status: string;
  pid?: number;
  description: string;
  detail: string;
  path?: string;
  command?: string;
  trigger?: string;
  duration: number;
  progress: number;
  startedAt?: string | null;
  updatedAt?: string | null;
  finishedAt?: string | null;
  error?: string;
};
