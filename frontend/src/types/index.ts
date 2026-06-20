export interface ImageDto {
  id: number;
  signature: string;
  extension: string;
  dominantColor: string;
  source?: string;
  artist?: Artist;
  artists: Artist[];
  uploadedAt: string;
  isNsfw: boolean;
  isAnimated: boolean;
  width: number;
  height: number;
  byteSize: number;
  url: string;
  tags: Tag[];
  albums?: AlbumDto[];
  favorites: number;
  likedAt?: string;
  uploaderId?: number;
  reviewStatus: ReviewStatus;
}

export interface Artist {
  id: number;
  name: string;
  patreon?: string;
  pixiv?: string;
  twitter?: string;
  deviantArt?: string;
  reviewStatus?: ReviewStatus;
  imageCount?: number;
  creatorId?: number;
}

export interface Tag {
  id: number;
  name: string;
  slug: string;
  description: string;
  reviewStatus?: ReviewStatus;
  imageCount?: number;
  creatorId?: number;
}
export interface User {
  id: number;
  name: string;
  role: Role;
  discordId?: string;
  avatarUrl?: string;
  isBlacklisted: boolean;
  blacklistReason?: string;
  requestCount?: number;
  apiKeyRequestCount?: number;
  jwtRequestCount?: number;
  uploadedImageCount?: number;
  albumImageCount?: number;
}

export interface AlbumDto {
  id: number;
  name: string;
  description: string;
  isDefault: boolean;
  imageCount?: number;
}

export interface ApiKeyDto {
  id: number;
  key?: string; // Only present once at creation time
  description: string;
  createdAt: string;
  lastUsedAt?: string;
  expirationDate?: string;
}

// Matches backend ReportStatus enum
export enum ReportStatus {
  Pending = 'Pending',
  Resolved = 'Resolved',
  Rejected = 'Rejected',
  Cancelled = 'Cancelled',
}

export interface Report {
  id: number;
  userId: number;
  user?: User;
  imageId?: number | null;
  image?: ImageDto;
  description?: string;
  status: ReportStatus;
  /** Optional moderator note recorded when the report was answered (validated/rejected). */
  reviewerNote?: string;
  createdAt: string;
}

export enum Role {
  User = 'User',
  TrustedUser = 'TrustedUser',
  Moderator = 'Moderator',
  Admin = 'Admin'
}

export enum ReviewStatus {
  Pending = 'Pending',
  Accepted = 'Accepted',
  Rejected = 'Rejected'
}

// Matches backend ReviewStatusFilter enum
export enum ReviewStatusFilter {
  Accepted = 'Accepted',
  Pending = 'Pending',
  All = 'All'
}

// Matches backend NsfwMode enum
export enum NsfwMode {
  False = 'False',
  True = 'True',
  All = 'All'
}

// Matches backend AnimatedMode enum
export enum AnimatedMode {
  False = 'False',
  True = 'True',
  All = 'All'
}

// Matches backend Orientation enum
export enum Orientation {
  All = 'All',
  Landscape = 'Landscape',
  Portrait = 'Portrait',
  Square = 'Square'
}

// Matches backend ImageOrderBy enum
export enum ImageOrderBy {
  Random = 'Random',
  UploadedAt = 'UploadedAt',
  Favorites = 'Favorites',
  AddedToAlbum = 'AddedToAlbum'
}

export interface PaginatedList<T> {
  items: T[];
  pageNumber: number;
  totalPages: number;
  totalCount: number;
  hasPreviousPage: boolean;
  hasNextPage: boolean;
  maxPageSize: number;
  defaultPageSize: number;
}

export interface ImageFormData {
  file?: File;  // Optional file replacement
  source?: string;
  isNsfw: boolean;
  tags: string[];
  artists: number[];
  userId?: number;
  reviewStatus?: ReviewStatus;
}

export interface PublicStats {
    totalRequests: number;
    totalImages: number;
    totalTags: number;
    totalArtists: number;
}

/**
 * ASP.NET Core ProblemDetails response format
 * @see https://tools.ietf.org/html/rfc7807
 */
export interface ProblemDetails {
    type?: string;
    title?: string;
    status?: number;
    detail?: string;
    instance?: string;
    /** Validation errors by field name */
    errors?: Record<string, string[]>;
    /** Generic message field (non-standard) */
    message?: string;
}

// Matches backend ReviewTaskStatus enum
export enum ReviewTaskStatus {
  Pending = 'Pending',
  Approved = 'Approved',
  Rejected = 'Rejected',
  Cancelled = 'Cancelled'
}

// Matches backend ReviewTaskKind enum
export enum ReviewTaskKind {
  NewContent = 'NewContent',
  Edit = 'Edit'
}

// Matches backend ReviewableContentType enum
export enum ReviewableContentType {
  Image = 'Image',
  Tag = 'Tag',
  Artist = 'Artist'
}

export interface UserMinimal {
  id: number;
  name: string;
  avatarUrl?: string;
}

/** The current state of a review task's target. Exactly one field is populated (all empty if deleted). */
export interface ReviewTaskTarget {
  image?: ImageDto;
  tag?: Tag;
  artist?: Artist;
}

/** Proposed changes to an image (only changed fields are present). Tag/artist membership uses deltas. */
export interface ImageEditPayload {
  source?: string;
  isNsfw?: boolean;
  addTagSlugs?: string[];
  removeTagSlugs?: string[];
  addArtistIds?: number[];
  removeArtistIds?: number[];
}

export interface TagEditPayload {
  name?: string;
  description?: string;
}

export interface ArtistEditPayload {
  name?: string;
  patreon?: string;
  pixiv?: string;
  twitter?: string;
  deviantArt?: string;
}

export type EditPayload = ImageEditPayload | TagEditPayload | ArtistEditPayload;

export interface ReviewTask {
  id: number;
  kind: ReviewTaskKind;
  submitterId?: number;
  submitter?: UserMinimal;
  targetType: ReviewableContentType;
  targetId: number;
  /** The proposed changes for Edit tasks (absent for NewContent). */
  payload?: EditPayload;
  /** The current state of the target; undefined if it no longer exists (e.g. a rejected submission). */
  target?: ReviewTaskTarget;
  reason?: string;
  status: ReviewTaskStatus;
  reviewerId?: number;
  reviewer?: UserMinimal;
  reviewerNote?: string;
  /** Set when a moderator (other than the submitter) edited this submission's content. */
  moderatorEditedAt?: string;
  createdAt: string;
  resolvedAt?: string;
}

/** Body sent to POST /review/tasks (submit an edit). */
export interface SubmitEditBody {
  targetType: ReviewableContentType;
  targetId: number;
  reason?: string;
  payload: EditPayload;
}