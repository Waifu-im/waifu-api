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
  favorites: number;
  likedAt?: string;
}

export interface Artist {
  id: number;
  name: string;
  patreon?: string;
  pixiv?: string;
  twitter?: string;
  deviantArt?: string;
}

export interface Tag {
  id: number;
  name: string;
  description: string;
  isNsfw: boolean;
}

export interface User {
  id: number;
  name: string;
  email?: string;
  role: Role;
}

export enum Role {
  User = 0,
  TrustedUser = 1,
  Moderator = 2,
  Admin = 3
}
