import about from "./about";
import audit from "./audit";
import scripts from "./scripts";
import system from "./system";
import zfs from "./zfs";
import ups from "./ups";
import balance from "./balance";
import common from "./common";
import console from "./console";
import containers from "./containers";
import dashboard from "./dashboard";
import docker from "./docker";
import errors from "./errors";
import gpus from "./gpus";
import history from "./history";
import layout from "./layout";
import location from "./location";
import login from "./login";
import logs from "./logs";
import network from "./network";
import notifications from "./notifications";
import plugins from "./plugins";
import settings from "./settings";
import storage from "./storage";
import terminal from "./terminal";
import tools from "./tools";
import tasks from "./tasks";
import users from "./users";
import vms from "./vms";

export const fr: Record<string, string> = {
  ...common,
  ...layout,
  ...dashboard,
  ...storage,
  ...docker,
  ...containers,
  ...vms,
  ...console,
  ...network,
  ...gpus,
  ...users,
  ...notifications,
  ...plugins,
  ...logs,
  ...terminal,
  ...tools,
  ...tasks,
  ...history,
  ...balance,
  ...settings,
  ...location,
  ...login,
  ...errors,
  ...about,
  ...audit,
  ...scripts,
  ...zfs,
  ...ups,
  ...system,
};
