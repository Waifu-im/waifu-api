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
        query = query.Where(i => i.ReviewStatus == ReviewStatus.Accepted);

        switch (filters.IsNsfw)
        {
            case NsfwMode.Safe:
                query = query.Where(i => i.IsNsfw == false);
                break;
            case NsfwMode.Nsfw:
                query = query.Where(i => i.IsNsfw == true);
                break;
            case NsfwMode.All:
                break;
        }

        if (filters.IsAnimated.HasValue)
        {
            query = query.Where(i => i.IsAnimated == filters.IsAnimated.Value);
        }

        if (!string.IsNullOrEmpty(filters.Orientation))
        {
            if (filters.Orientation.Equals("LANDSCAPE", StringComparison.OrdinalIgnoreCase))
                query = query.Where(i => i.Width > i.Height);
            else if (filters.Orientation.Equals("PORTRAIT", StringComparison.OrdinalIgnoreCase))
                query = query.Where(i => i.Height > i.Width);
            else if (filters.Orientation.Equals("SQUARE", StringComparison.OrdinalIgnoreCase))
                query = query.Where(i => i.Width == i.Height);
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