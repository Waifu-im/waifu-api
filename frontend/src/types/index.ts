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
  keyPrefix: string;
  description: string; // Changed from label to match backend
  createdAt: string;
  lastUsedAt?: string;
  expirationDate?: string; // Changed from expiresAt to match backend
}

export interface Report {
  id: number;
  userId: number;
  user?: User;
  imageId: number;
  image?: ImageDto;
  description?: string;
  isResolved: boolean;
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