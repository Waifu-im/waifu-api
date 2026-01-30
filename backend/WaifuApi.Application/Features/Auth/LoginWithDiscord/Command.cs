using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Auth.LoginWithDiscord;

public record LoginWithDiscordCommand(string Code) : ICommand<string>;

public class LoginWithDiscordCommandHandler : ICommandHandler<LoginWithDiscordCommand, string>
{
    private readonly IWaifuDbContext _context;
    private readonly ITokenService _tokenService;
    private readonly IDiscordService _discordService;

    public LoginWithDiscordCommandHandler(
        IWaifuDbContext context,
        ITokenService tokenService,
        IDiscordService discordService)
    {
        _context = context;
        _tokenService = tokenService;
        _discordService = discordService;
    }

    public async ValueTask<string> Handle(LoginWithDiscordCommand request, CancellationToken cancellationToken)
    {
        var accessToken = await _discordService.GetAccessTokenAsync(request.Code);
        var discordUser = await _discordService.GetUserProfileAsync(accessToken);

        string? avatarUrl = null;
        if (!string.IsNullOrEmpty(discordUser.Avatar))
        {
            avatarUrl = $"https://cdn.discordapp.com/avatars/{discordUser.Id}/{discordUser.Avatar}.png";
        }

        // Find or create user
        var user = await _context.Users.FirstOrDefaultAsync(u => u.DiscordId == discordUser.Id, cancellationToken);
        if (user == null)
        {
            user = new User
            {
                DiscordId = discordUser.Id,
                Name = discordUser.Username,
                AvatarUrl = avatarUrl,
                Role = Role.User
            };
            _context.Users.Add(user);
            
            // Create default Favorites album
            var favoritesAlbum = new Album
            {
                Name = "Favorites",
                Description = "Your favorite images",
                IsDefault = true,
                User = user
            };
            _context.Albums.Add(favoritesAlbum);
        }
        else
        {
            // Update info if needed
            user.Name = discordUser.Username;
            user.AvatarUrl = avatarUrl;
        }

        await _context.SaveChangesAsync(cancellationToken);

        return _tokenService.GenerateJwtToken(user);
    }
}
