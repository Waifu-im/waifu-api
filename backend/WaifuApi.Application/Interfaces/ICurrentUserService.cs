using System.Threading.Tasks;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Interfaces;

public interface ICurrentUserService
{
    long? UserId { get; }
    bool IsAuthenticated { get; }
    Role? UserRole { get; }
    bool IsModeratorOrAdmin { get; }
    bool IsAdmin { get; }
    Task<long> ResolveUserIdAsync(string userIdStr);
    Task<long?> ResolveAlbumIdAsync(long userId, string? albumIdStr);
}
