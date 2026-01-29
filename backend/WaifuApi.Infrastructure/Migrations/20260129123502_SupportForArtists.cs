using Microsoft.EntityFrameworkCore.Migrations;

#nullable disable

namespace WaifuApi.Infrastructure.Migrations
{
    /// <inheritdoc />
    public partial class SupportForArtists : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropForeignKey(
                name: "FK_Images_Artists_ArtistId",
                table: "Images");

            migrationBuilder.DropIndex(
                name: "IX_Images_ArtistId",
                table: "Images");

            migrationBuilder.DropColumn(
                name: "ArtistId",
                table: "Images");

            migrationBuilder.CreateTable(
                name: "ArtistImage",
                columns: table => new
                {
                    ArtistsId = table.Column<long>(type: "bigint", nullable: false),
                    ImagesId = table.Column<long>(type: "bigint", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_ArtistImage", x => new { x.ArtistsId, x.ImagesId });
                    table.ForeignKey(
                        name: "FK_ArtistImage_Artists_ArtistsId",
                        column: x => x.ArtistsId,
                        principalTable: "Artists",
                        principalColumn: "Id",
                        onDelete: ReferentialAction.Cascade);
                    table.ForeignKey(
                        name: "FK_ArtistImage_Images_ImagesId",
                        column: x => x.ImagesId,
                        principalTable: "Images",
                        principalColumn: "Id",
                        onDelete: ReferentialAction.Cascade);
                });

            migrationBuilder.CreateIndex(
                name: "IX_ArtistImage_ImagesId",
                table: "ArtistImage",
                column: "ImagesId");
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropTable(
                name: "ArtistImage");

            migrationBuilder.AddColumn<long>(
                name: "ArtistId",
                table: "Images",
                type: "bigint",
                nullable: true);

            migrationBuilder.CreateIndex(
                name: "IX_Images_ArtistId",
                table: "Images",
                column: "ArtistId");

            migrationBuilder.AddForeignKey(
                name: "FK_Images_Artists_ArtistId",
                table: "Images",
                column: "ArtistId",
                principalTable: "Artists",
                principalColumn: "Id",
                onDelete: ReferentialAction.SetNull);
        }
    }
}
