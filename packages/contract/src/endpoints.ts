import { apiV1 } from "./index";

export const HEALTHZ = apiV1("healthz");
export const META = apiV1("meta");

export const SESSIONS = apiV1("sessions");
export const SESSION_EVENTS = (id: string) => apiV1("sessions", id, "events");
export const SESSION_MESSAGES = (id: string) => apiV1("sessions", id, "messages");
export const SESSION_PROMPT = (id: string) => apiV1("sessions", id, "prompt");
export const SESSION_ABORT = (id: string) => apiV1("sessions", id, "abort");
export const SESSION_FORK = (id: string) => apiV1("sessions", id, "fork");
export const SESSION_TITLE = (id: string) => apiV1("sessions", id, "title");

export const LOGIN = apiV1("login");
export const REFRESH = apiV1("refresh");
export const LOGOUT = apiV1("logout");

export const DEVICES = apiV1("devices");
export const DEVICE_BY_ID = (id: string) => apiV1("devices", id);

export const PAIRING_INIT = apiV1("pairing", "init");
export const PAIRING_REDEEM = apiV1("pairing", "redeem");

export const SSH_KEYS = apiV1("ssh", "keys");
export const SSH_KEYS_BY_ID = (id: string) => apiV1("ssh", "keys", id);
export const SSH_SERVERS = apiV1("ssh", "servers");
export const SSH_SERVERS_BY_ID = (id: string) => apiV1("ssh", "servers", id);
export const SSH_SERVERS_TEST = (id: string) => apiV1("ssh", "servers", id, "test");

export const ONBOARDING_STATUS = apiV1("onboarding", "status");
