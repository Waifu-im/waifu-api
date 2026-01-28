using System;
using System.Security.Claims;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Http;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Web.Services;

public class CurrentUserService : ICurrentUserService
{
    private readonly IHttpContextAccessor _httpContextAccessor;
    private readonly IWaifuDbContext _context;

    public CurrentUserService(IHttpContextAccessor httpContextAccessor, IWaifuDbContext context)
    {
        _httpContextAccessor = httpContextAccessor;
        _context = context;
    }

    public long? UserId
    {
        get
        {
            var idClaim = _httpContextAccessor.HttpContext?.User?.FindFirst(ClaimTypes.NameIdentifier);
            if (idClaim != null && long.TryParse(idClaim.Value, out var id))
            {
                return id;
            }
            return null;
        }
    }

    public async Task<long> ResolveUserIdAsync(string userIdStr)
    {
        if (userIdStr.Equals("me", StringComparison.OrdinalIgnoreCase))
        {
            var id = UserId;
            if (id == null) throw new UnauthorizedAccessException("User is not authenticated.");
            return id.Value;
        }
        
        if (long.TryParse(userIdStr, out var parsedId))
        {
            return parsedId;
        }

        throw new ArgumentException("Invalid User ID format.");
    }

    public async Task<long?> ResolveAlbumIdAsync(long userId, string? albumIdStr)
    {
        if (string.IsNullOrEmpty(albumIdStr)) return null;

        if (albumIdStr.Equals("favorites", StringComparison.OrdinalIgnoreCase))
        {
            var album = await _context.Albums.FirstOrDefaultAsync(a => a.UserId == userId && a.IsDefault);
            if (album == null)
            {
                // Lazy creation
                album = new Album { UserId = userId, Name = "Favorites", IsDefault = true };
                _context.Albums.Add(album);
                await _context.SaveChangesAsync(System.Threading.CancellationToken.None);
            }
            return album.Id;
        }

        if (long.TryParse(albumIdStr, out var parsedId))
        {
            return parsedId;
        }

        throw new ArgumentException("Invalid Album ID format.");
    }
}
