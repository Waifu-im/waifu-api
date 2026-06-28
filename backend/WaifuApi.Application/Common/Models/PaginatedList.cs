using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Utilities;

namespace WaifuApi.Application.Common.Models;

public class PaginatedList<T>
{
    public List<T> Items { get; }
    public int PageNumber { get; }
    public int TotalPages { get; }
    public int TotalCount { get; }
    public int MaxPageSize { get; }
    public int DefaultPageSize { get; }

    public PaginatedList(List<T> items, int count, int pageNumber, int pageSize, int maxPageSize, int defaultPageSize)
    {
        var (normalizedPage, normalizedPageSize) = PaginationUtils.Normalize(pageNumber, pageSize, defaultPageSize, maxPageSize);
        PageNumber = normalizedPage;
        TotalPages = (int)Math.Ceiling(count / (double)normalizedPageSize);
        TotalCount = count;
        Items = items;
        MaxPageSize = maxPageSize;
        DefaultPageSize = defaultPageSize;
    }

    public bool HasPreviousPage => PageNumber > 1;

    public bool HasNextPage => PageNumber < TotalPages;

    public static async Task<PaginatedList<T>> CreateAsync(IQueryable<T> source, int pageNumber, int pageSize, int maxPageSize, int defaultPageSize, CancellationToken cancellationToken)
    {
        var (normalizedPage, normalizedPageSize) = PaginationUtils.Normalize(pageNumber, pageSize, defaultPageSize, maxPageSize);
        var count = await source.CountAsync(cancellationToken);
        var items = await source.Skip((normalizedPage - 1) * normalizedPageSize).Take(normalizedPageSize).ToListAsync(cancellationToken);

        return new PaginatedList<T>(items, count, normalizedPage, normalizedPageSize, maxPageSize, defaultPageSize);
    }
}
