namespace WaifuApi.Application.Common.Utilities;

public static class PaginationUtils
{
    public static (int Page, int PageSize) Normalize(int page, int pageSize, int defaultPageSize, int maxPageSize)
    {
        if (pageSize <= 0) pageSize = defaultPageSize;
        if (maxPageSize > 0 && pageSize > maxPageSize) pageSize = maxPageSize;
        if (page < 1) page = 1;
        return (page, pageSize);
    }
}
