using System.Threading.Tasks;

namespace WaifuApi.Application.Interfaces;

public interface IDiscordService
{
    Task<string> GetAccessTokenAsync(string code);
    Task<DiscordUserDto> GetUserProfileAsync(string accessToken);
}

public class DiscordUserDto
{
    public string Id { get; set; } = string.Empty;
    public string Username { get; set; } = string.Empty;
    public string? Email { get; set; }
}
