export interface InstalledInfo {
  installed: boolean;
  path: string;
}

export interface Status {
  server_name: string;
  server_version: string;
  vault_path: string;
  project_code: string;
  taolu_count: number;
  archived_count: number;
  groups: string[];
  domains: string[];
  user_domain: string;
  authoring: string;
  uptime: string;
  installed: Record<string, InstalledInfo>;
}

export interface Config {
  user_domain: string;
}

export interface TaoluItem {
  name: string;
  group: string;
  domain: string;
  mode: string;
  description: string;
  tags: string[];
  latest_version: string;
  archived: boolean;
}

export interface AssetMeta {
  path: string;
}

export interface TaoluDetail {
  name: string;
  group: string;
  domain: string;
  mode: string;
  archived: boolean;
  skill: string;
  action: string;
  assets: AssetMeta[];
  latest_version: string;
  version_count: number;
}

export interface ContentFile {
  path: string;
  content: string;
}

export interface ContentResponse {
  name: string;
  version: string;
  archived: boolean;
  files: ContentFile[];
  asset_count: number;
}

export interface Version {
  label: string;
  uuid: string;
  date: string;
  user: string;
  message: string;
}

export interface DiffFile {
  path: string;
  unified: string;
}

export interface DiffResponse {
  name: string;
  version_a: string;
  version_b: string;
  files: DiffFile[];
}
