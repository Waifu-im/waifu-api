using System.Threading.Tasks;

namespace WaifuApi.Application.Interfaces;

public interface ICurrentUserService
{
    long? UserId { get; }
    Task<long> ResolveUserIdAsync(string userIdStr);
    Task<long?> ResolveAlbumIdAsync(long userId, string? albumIdStr);
}
