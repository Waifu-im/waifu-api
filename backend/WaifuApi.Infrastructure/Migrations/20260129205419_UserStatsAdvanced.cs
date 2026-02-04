using Microsoft.EntityFrameworkCore.Migrations;

#nullable disable

namespace WaifuApi.Infrastructure.Migrations
{
    /// <inheritdoc />
    public partial class UserStatsAdvanced : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.AddColumn<long>(
                name: "ApiKeyRequestCount",
                table: "Users",
                type: "bigint",
                nullable: false,
                defaultValue: 0L);

            migrationBuilder.AddColumn<long>(
                name: "JwtRequestCount",
                table: "Users",
                type: "bigint",
                nullable: false,
                defaultValue: 0L);
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropColumn(
                name: "ApiKeyRequestCount",
                table: "Users");

            migrationBuilder.DropColumn(
                name: "JwtRequestCount",
                table: "Users");
        }
    }
}
