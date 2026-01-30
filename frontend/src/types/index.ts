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
  uploaderId: number;
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
}

export interface Tag {
  id: number;
  name: string;
  slug: string;
  description: string;
  reviewStatus?: ReviewStatus;
}
export interface User {
  id: number;
  name: string;
  role: Role;
  discordId?: string;
  avatarUrl?: string;
  isBlacklisted: boolean;
  requestCount?: number;
  apiKeyRequestCount?: number;
  jwtRequestCount?: number;
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
  User = 0,
  TrustedUser = 1,
  Moderator = 2,
  Admin = 3
}

export enum ReviewStatus {
  Pending = 0,
  Accepted = 1,
  Rejected = 2
}

export enum ImageSort {
  ADDED_TO_ALBUM = 'ADDED_TO_ALBUM',
  UPLOADED_AT = 'UPLOADED_AT',
  FAVORITES = 'FAVORITES',
  RANDOM = 'RANDOM'
}

export interface PaginatedList<T> {
  items: T[];
  pageNumber: number;
  totalPages: number;
  totalCount: number;
  hasPreviousPage: boolean;
  hasNextPage: boolean;
}

export interface ImageFormData {
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