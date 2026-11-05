export type Level = "national" | "province" | "municipality" | "ward" | "vd";
export type CouncilStatus = "pending" | "majority" | "hung" | "drawn";

export interface Party {
  id: number;
  name: string;
  votes: number;
  pct: number;
  wardVotes?: number;
  prVotes?: number;
  wardSeats?: number;
  prSeats?: number;
  seats?: number;
  largest?: number;
  controlled?: number;
  leadVotes?: boolean;
  leadSeats?: boolean;
  candidate?: string;
}

export interface Progress {
  vds: number; vdsExpected: number;
  wards: number; wardsExpected: number;
  seats: number; seatsExpected: number;
  councils: number; councilsExpected: number;
}

export interface Votes { registered: number; cast: number; valid: number; spoilt: number; special: number }
export interface Tally { hung: number; majority: number; drawn: number; pending: number }
export interface Council {
  status: CouncilStatus;
  controlParty?: number;
  largestParty?: number;
  largestTied?: boolean;
  majorityNeeded: number;
  seatsAllocated: number;
  seatsTotal: number;
}

export interface Area {
  level: Level;
  id: number;
  parentId?: number;
  name: string;
  code?: string;
  isMetro?: boolean;
  reportDate: string;
  progress: Progress;
  votes: Votes;
  tally?: Tally;
  council?: Council;
  parties: Party[];
}

export interface AreaSummary {
  id: number;
  name: string;
  code?: string;
  parentId?: number;
  isMetro?: boolean;
  reported: boolean;
  leaderId?: number;
  leaderPct?: number;
  valid?: number;
  vdPct?: number;
  status?: CouncilStatus;
  partyPct?: number;
  partyVotes?: number;
  partySwing?: number;
  winnerId?: number;
}

export interface NationalView extends Area { provinces: AreaSummary[] }
export interface ProvinceView extends Area { municipalities: AreaSummary[] }
export interface MunicipalityView extends Area { provinceName: string; wards: AreaSummary[] }

export interface AreaRef { level: Level; id: number; name: string; parentId?: number; isMetro?: boolean }

export interface WardView {
  id: number; no: number; municipality: AreaRef; provinceName: string; reportDate: string;
  reported: boolean; winnerId?: number; candidate?: string; parties: Party[]; vds: AreaSummary[];
}
export interface VDView {
  id: number; name: string; wardId: number; wardNo: number; municipality: AreaRef; provinceName: string;
  reportDate: string; reported: boolean; parties: Party[];
}
export interface CouncilRow {
  id: number; name: string; code: string; province: number; isMetro: boolean; council: Council; vdPct: number; parties: Party[];
}
export interface PartyInfo { id: number; name: string; abbr: string; color: string }
export interface Status {
  ready: boolean; replay?: boolean; seq: number; lastIecAnswer: string; lastChange: string; pausedUntil: string; stale: boolean;
}
export interface FeedEvent<T = any> { seq: number; type: string; at: string; data: T }
export interface SearchHit { level: Level; id: number; name: string; context?: string; municId?: number; wardId?: number }
export interface TimelinePoint { at: string; progress: Progress; parties: Party[] }
export interface PartySeries { partyId: number; abbr: string; years: { year: number; pct: number }[]; current: number | null; swing: number | null }
export interface HistoryView { level: Level; id: number; name: string; comparable: boolean; note?: string; parties: PartySeries[] }
