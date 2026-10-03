import type { components } from './schema'

export type User = components['schemas']['User']
export type AuthResult = components['schemas']['AuthResult']
export type Dashboard = components['schemas']['Dashboard']
export type Group = components['schemas']['Group']
export type Service = components['schemas']['Service']
export type ServiceInput = Required<components['schemas']['ServiceInput']>
export type ServicePatch = components['schemas']['ServicePatch']
export type Layout = components['schemas']['Layout']
export type Credentials = components['schemas']['Credentials']
export type SetupInput = components['schemas']['SetupInput']
export type Settings = components['schemas']['Settings']
export type SettingsPatch = components['schemas']['SettingsPatch']
export type ImportFile = components['schemas']['ImportFile']
export type ImportInput = components['schemas']['ImportInput']
export type DockerScanInput = components['schemas']['DockerScanInput']
export type ImportPreview = components['schemas']['ImportPreview']
export type ImportSelection = Required<components['schemas']['ImportSelection']>
export type ImportCommit = components['schemas']['ImportCommit']
export type ImportResult = components['schemas']['ImportResult']
export type WebStatus = components['schemas']['WebStatus']
