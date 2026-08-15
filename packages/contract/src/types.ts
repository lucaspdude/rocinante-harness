export interface HealthResponse {
  ok: boolean;
}

export interface MetaResponse {
  api_version: string;
  omp_version: string;
  handshake_version: 1 | 2;
}

export interface SessionCreateRequest {
  omp_cwd: string;
  cwd: string;
  model?: string;
}

export interface SessionCreateResponse {
  id: string;
  omp_cwd: string;
  cwd: string;
  created_at: string;
}

export interface PromptRequest {
  content: string;
  model?: string;
}

export interface PromptResponse {
  client_request_id: string;
  received_at: string;
}

export interface ForkRequest {
  at_message_id: string;
}

export interface ForkResponse {
  id: string;
}

export interface TitleRequest {
  title: string;
}

export interface LoginRequest {
  passphrase: string;
  device_name?: string;
}

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
  device_id: string;
  expires_in: number;
}

export interface RefreshRequest {
  refresh_token: string;
}

export interface RefreshResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

export interface Device {
  id: string;
  name: string;
  created_at: string;
  last_used_at: string;
  current: boolean;
}

export interface PairingInitResponse {
  code: string;
  expires_at: string;
}

export interface PairingRedeemRequest {
  code: string;
  device_name?: string;
}

export interface OnboardingStatusResponse {
  initialized: boolean;
  requires_setup: boolean;
  api_version: string;
}
