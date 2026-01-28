using System;
using System.Linq;
using WaifuApi.Application.Common.Models;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Extensions;

public static class ImageQueryExtensions
{
    public static IQueryable<Image> ApplyFilters(this IQueryable<Image> query, ImageFilters filters)
    {
        // Filter by ReviewStatus
        query = query.Where(i => i.ReviewStatus == ReviewStatus.Accepted);

        // IsNsfw Logic
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

        // IsAnimated Logic
        if (filters.IsAnimated.HasValue)
        {
            if (filters.IsAnimated.Value)
            {
                query = query.Where(i => i.IsAnimated == true);
            }
            else
            {
                query = query.Where(i => i.IsAnimated == false);
            }
        }

        if (filters.IncludedTags.Any())
        {
            foreach (var tag in filters.IncludedTags)
            {
                query = query.Where(i => i.Tags.Any(t => t.Name == tag));
            }
        }

        if (filters.ExcludedTags.Any())
        {
            foreach (var tag in filters.ExcludedTags)
            {
                query = query.Where(i => !i.Tags.Any(t => t.Name == tag));
            }
        }

        return query;
    }
}
