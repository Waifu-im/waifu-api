using System;
using System.Linq;
using System.Linq.Expressions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Extensions;

public static class ImageQueryExtensions
{
    public static IQueryable<Image> ApplyFilters(this IQueryable<Image> query, ImageFilters filters)
    {
        // If AlbumId is present, we don't filter by ReviewStatus here because the album logic handles it (or should show all in album regardless of status if that's the intent, but usually albums contain accepted images unless it's a personal album where pending might be allowed? The prompt says "pending image should be displayed in albums. Only in albums not regular Getimage")
        // Actually, if we are in an album context, we might want to override the default "Accepted only" behavior.
        // The prompt says: "pending image should be displayed in albums. Only in albums not regular Getimage"
        
        if (filters.AlbumId.HasValue)
        {
            // Albums: only filter if a specific status is requested, otherwise show all statuses
            switch (filters.ReviewStatus)
            {
                case ReviewStatusFilter.Pending:
                    query = query.Where(i => i.ReviewStatus == ReviewStatus.Pending);
                    break;
                case ReviewStatusFilter.Accepted:
                    query = query.Where(i => i.ReviewStatus == ReviewStatus.Accepted);
                    break;
                case ReviewStatusFilter.All:
                    break;
            }
        }
        else
        {
            // Regular GetImages (Gallery): default Accepted
            switch (filters.ReviewStatus)
            {
                case ReviewStatusFilter.Pending:
                    query = query.Where(i => i.ReviewStatus == ReviewStatus.Pending);
                    break;
                case ReviewStatusFilter.All:
                    break;
                default:
                    query = query.Where(i => i.ReviewStatus == ReviewStatus.Accepted);
                    break;
            }
        }

        switch (filters.IsNsfw)
        {
            case NsfwMode.False:
                query = query.Where(i => i.IsNsfw == false);
                break;
            case NsfwMode.True:
                query = query.Where(i => i.IsNsfw == true);
                break;
            case NsfwMode.All:
                break;
        }

        switch (filters.IsAnimated)
        {
            case AnimatedMode.True:
                query = query.Where(i => i.IsAnimated == true);
                break;
            case AnimatedMode.False:
                query = query.Where(i => i.IsAnimated == false);
                break;
            case AnimatedMode.All:
                break;
        }

        switch (filters.Orientation)
        {
            case Orientation.Landscape:
                query = query.Where(i => i.Width > i.Height);
                break;
            case Orientation.Portrait:
                query = query.Where(i => i.Height > i.Width);
                break;
            case Orientation.Square:
                query = query.Where(i => i.Width == i.Height);
                break;
            case Orientation.All:
                break;
        }

        if (filters.IncludedTags.Any())
        {
            foreach (var tagInput in filters.IncludedTags)
            {
                if (long.TryParse(tagInput, out var tagId))
                {
                    query = query.Where(i => i.Tags.Any(t => t.Id == tagId));
                }
                else
                {
                    var slug = tagInput.Trim().ToLowerInvariant();
                    query = query.Where(i => i.Tags.Any(t => t.Slug == slug));
                }
            }
        }

        if (filters.ExcludedTags.Any())
        {
            foreach (var tagInput in filters.ExcludedTags)
            {
                if (long.TryParse(tagInput, out var tagId))
                {
                    query = query.Where(i => !i.Tags.Any(t => t.Id == tagId));
                }
                else
                {
                    var slug = tagInput.Trim().ToLowerInvariant();
                    query = query.Where(i => !i.Tags.Any(t => t.Slug == slug));
                }
            }
        }
        
        if (filters.IncludedArtists.Any())
        {
            foreach (var artistInput in filters.IncludedArtists)
            {
                if (long.TryParse(artistInput, out var artistId))
                {
                    query = query.Where(i => i.Artists.Any(a => a.Id == artistId));
                }
            }
        }
        
        if (filters.ExcludedArtists.Any())
        {
            foreach (var artistInput in filters.ExcludedArtists)
            {
                if (long.TryParse(artistInput, out var artistId))
                {
                    query = query.Where(i => !i.Artists.Any(a => a.Id == artistId));
                }
            }
        }

        if (filters.IncludedIds.Any())
        {
            foreach (var idInput in filters.IncludedIds)
            {
                if (long.TryParse(idInput, out var id))
                {
                    query = query.Where(i => i.Id == id);
                }
            }
        }

        if (filters.ExcludedIds.Any())
        {
            foreach (var idInput in filters.ExcludedIds)
            {
                if (long.TryParse(idInput, out var id))
                {
                    query = query.Where(i => i.Id != id);
                }
            }
        }

        query = ApplyRangeFilter(query, filters.Width, i => i.Width);
        query = ApplyRangeFilter(query, filters.Height, i => i.Height);
        query = ApplyRangeFilter(query, filters.ByteSize, i => i.ByteSize);

        if (filters.UploaderId.HasValue)
        {
            query = query.Where(i => i.UploaderId == filters.UploaderId.Value);
        }

        return query;
    }

    private static IQueryable<Image> ApplyRangeFilter(IQueryable<Image> query, string filter, Expression<Func<Image, long>> propertySelector)
    {
        if (string.IsNullOrWhiteSpace(filter)) return query;
        filter = filter.Trim();
        long value;

        if (filter.StartsWith(">="))
        {
            if (long.TryParse(filter.Substring(2), out value))
            {
                var body = Expression.GreaterThanOrEqual(propertySelector.Body, Expression.Constant(value));
                var lambda = Expression.Lambda<Func<Image, bool>>(body, propertySelector.Parameters);
                return query.Where(lambda);
            }
        }
        else if (filter.StartsWith(">"))
        {
            if (long.TryParse(filter.Substring(1), out value))
            {
                var body = Expression.GreaterThan(propertySelector.Body, Expression.Constant(value));
                var lambda = Expression.Lambda<Func<Image, bool>>(body, propertySelector.Parameters);
                return query.Where(lambda);
            }
        }
        else if (filter.StartsWith("<="))
        {
            if (long.TryParse(filter.Substring(2), out value))
            {
                var body = Expression.LessThanOrEqual(propertySelector.Body, Expression.Constant(value));
                var lambda = Expression.Lambda<Func<Image, bool>>(body, propertySelector.Parameters);
                return query.Where(lambda);
            }
        }
        else if (filter.StartsWith("<"))
        {
            if (long.TryParse(filter.Substring(1), out value))
            {
                var body = Expression.LessThan(propertySelector.Body, Expression.Constant(value));
                var lambda = Expression.Lambda<Func<Image, bool>>(body, propertySelector.Parameters);
                return query.Where(lambda);
            }
        }
        else if (filter.Contains(".."))
        {
            var parts = filter.Split("..");
            if (parts.Length == 2 && long.TryParse(parts[0], out var min) && long.TryParse(parts[1], out var max))
            {
                var bodyMin = Expression.GreaterThanOrEqual(propertySelector.Body, Expression.Constant(min));
                var bodyMax = Expression.LessThanOrEqual(propertySelector.Body, Expression.Constant(max));
                var body = Expression.AndAlso(bodyMin, bodyMax);
                var lambda = Expression.Lambda<Func<Image, bool>>(body, propertySelector.Parameters);
                return query.Where(lambda);
            }
        }
        else if (long.TryParse(filter, out value))
        {
            var body = Expression.Equal(propertySelector.Body, Expression.Constant(value));
            var lambda = Expression.Lambda<Func<Image, bool>>(body, propertySelector.Parameters);
            return query.Where(lambda);
        }

        return query;
    }
}