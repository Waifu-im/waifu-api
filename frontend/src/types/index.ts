export interface ImageDto {
  id: number;
  signature: string;
  extension: string;
  dominantColor: string;
  source?: string;
  artist?: Artist;
  uploadedAt: string;
  isNsfw: boolean;
  isAnimated: boolean;
  width: number;
  height: number;
  byteSize: number;
  url: string;
  tags: Tag[];
  albums?: AlbumDto[]; // Keeps your previous request
  favorites: number;
  likedAt?: string;
  uploaderId: number;
}

export interface Artist {
  id: number;
  name: string;
  patreon?: string;
  pixiv?: string;
  twitter?: string;
  deviantArt?: string;
  reviewStatus?: number; // Added to fix TS2339 in Review.tsx
}

export interface Tag {
  id: number;
  name: string;
  description: string;
  reviewStatus?: number; // Added to fix TS2339 in Review.tsx
}

export interface User {
  id: number;
  name: string;
  role: Role;
}

export interface AlbumDto {
  id: number;
  name: string;
  description: string;
  isDefault: boolean;
}

export interface ApiKeyDto {
  id: number;
  keyPrefix: string;
  label: string;
  createdAt: string;
  lastUsedAt?: string;
  expiresAt?: string;
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
  tagIds: number[];
  artistId?: number | null;
  userId?: number;
}